import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

root = Path(tempfile.mkdtemp(prefix="lazyslice-review-probes-"))
name = "lazyslice-review-" + str(os.getpid())

def run(args, **kwargs):
    return subprocess.run(args, text=True, capture_output=True, **kwargs)

def sql(db, query):
    p = run(["docker", "exec", "-i", name, "psql", "-U", "postgres", "-d", db, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1"], input=query)
    if p.returncode:
        raise RuntimeError(p.stderr)
    return p.stdout.strip()

cases = {
    "json_keys": (
        "CREATE TABLE items (id integer PRIMARY KEY, payload jsonb); INSERT INTO items VALUES (1, '{\"canary.person@example.org\": \"ok\"}');",
        "SELECT jsonb_object_keys(payload) FROM items;"
    ),
    "ddl_default": (
        "CREATE TABLE items (id integer PRIMARY KEY, email text DEFAULT 'ddl.canary@example.org'); INSERT INTO items VALUES (1,'row.canary@example.org');",
        "SELECT email FROM items; SELECT pg_get_expr(adbin,adrelid) FROM pg_attrdef WHERE adrelid='items'::regclass;"
    ),
    "sparse_email": (
        "CREATE TABLE items (id integer PRIMARY KEY, v text); INSERT INTO items SELECT i, CASE WHEN i=1 THEN 'sparse.canary@example.org' ELSE 'ok' END FROM generate_series(1,20) i;",
        "SELECT count(*) FROM items WHERE v='sparse.canary@example.org';"
    ),
    "verify_failure": (
        "CREATE TABLE items (id integer PRIMARY KEY, v text); INSERT INTO items VALUES (1,'verify.canary@example.org'),(2,'ok');",
        "SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1; SELECT v FROM items WHERE id=1;"
    ),
    "fk_masker": (
        "CREATE TABLE tokens (id integer PRIMARY KEY, token text UNIQUE); CREATE TABLE items (id integer PRIMARY KEY, token text REFERENCES tokens(token)); INSERT INTO tokens VALUES (1,'OriginalCredentialABC123'); INSERT INTO items VALUES (1,'OriginalCredentialABC123');",
        "SELECT token FROM tokens; SELECT token FROM items; SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1;"
    ),
    "polymorphic_log": (
        "CREATE TABLE items (id integer PRIMARY KEY, thing_id integer, thing_type text); INSERT INTO items VALUES (1,1,'poly.canary@example.org');",
        "SELECT thing_type FROM items;"
    ),
}

if len(sys.argv)>1:
    cases={key:cases[key] for key in sys.argv[1:]}

started = run(["docker", "run", "--rm", "-d", "--name", name, "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-p", "127.0.0.1::5432", "postgres:18"])
if started.returncode:
    raise RuntimeError(started.stderr)
try:
    for _ in range(100):
        if run(["docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"]).returncode == 0:
            break
        time.sleep(0.2)
    port = run(["docker", "port", name, "5432/tcp"]).stdout.strip().rsplit(":",1)[1]
    env = dict(os.environ, LAZYSLICE_SECRET="42"*32)
    results = []
    for case, (setup, query) in cases.items():
        source, target = "s_" + case, "t_" + case
        sql("postgres", "CREATE DATABASE " + source)
        sql("postgres", "CREATE DATABASE " + target)
        sql(source, setup)
        dsn = "postgres://postgres@127.0.0.1:" + port + "/"
        p = run(["/tmp/lazyslice-review", "--source", dsn+source+"?sslmode=disable", "--target", dsn+target+"?sslmode=disable", "--root", "public.items", "--take", "20", "--yes", "--no-config"], cwd=root, env=env, timeout=90)
        (root/(case+".log")).write_text(p.stdout+p.stderr)
        result = {"case":case, "exit":p.returncode, "target":sql(target,query), "log":str(root/(case+".log"))}
        results.append(result)
        print(json.dumps(result), flush=True)
    (root/"results.json").write_text(json.dumps(results,indent=2))
finally:
    run(["docker", "rm", "-f", name])
print("Evidence directory:", root)
