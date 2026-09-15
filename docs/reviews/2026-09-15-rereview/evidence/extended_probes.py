import json
import os
from pathlib import Path
import subprocess
import tempfile
import time


root = Path(tempfile.mkdtemp(prefix="lazyslice-rereview-extended-"))
name = "lazyslice-rereview-extended-" + str(os.getpid())
binary = os.environ.get("LAZYSLICE_REVIEW_BIN")
if not binary:
    raise RuntimeError("set LAZYSLICE_REVIEW_BIN to the pinned lazyslice binary")


def run(args, **kwargs):
    return subprocess.run(args, text=True, capture_output=True, **kwargs)


def sql(database, query, strict=True):
    result = run(
        [
            "docker", "exec", "-i", name, "psql", "-U", "postgres",
            "-d", database, "-XAt", "-v", "ON_ERROR_STOP=1",
        ],
        input=query,
    )
    if strict and result.returncode:
        raise RuntimeError(result.stderr)
    return {
        "exit": result.returncode,
        "out": result.stdout.strip(),
        "error": result.stderr.strip(),
    }


cases = {
    "matview_target": (
        "CREATE TABLE items(id int PRIMARY KEY); INSERT INTO items VALUES(1)",
        "CREATE TABLE items(id int PRIMARY KEY); INSERT INTO items VALUES(999); "
        "CREATE MATERIALIZED VIEW archive AS SELECT * FROM items; DELETE FROM items;",
        "SELECT to_regclass('archive'); SELECT * FROM items;",
    ),
    "citext_fk": (
        "CREATE EXTENSION citext; "
        "CREATE TABLE tokens(id int PRIMARY KEY,token citext UNIQUE); "
        "CREATE TABLE items(id int PRIMARY KEY,token citext REFERENCES tokens(token)); "
        "INSERT INTO tokens VALUES(1,'OriginalCredentialABC123'); "
        "INSERT INTO items VALUES(1,'originalcredentialabc123');",
        "",
        "SELECT token FROM tokens; SELECT token FROM items; "
        "SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1;",
    ),
    "date_fk": (
        "CREATE TABLE parents(id int PRIMARY KEY,birth_date date UNIQUE); "
        "CREATE TABLE items(id int PRIMARY KEY,birth_date timestamp REFERENCES parents(birth_date)); "
        "INSERT INTO parents VALUES(1,'1981-02-03'); "
        "INSERT INTO items VALUES(1,'1981-02-03');",
        "",
        "SELECT birth_date FROM parents; SELECT birth_date FROM items; "
        "SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1;",
    ),
    "numeric_fk": (
        "CREATE TABLE parents(id int PRIMARY KEY,tax_id numeric UNIQUE); "
        "CREATE TABLE items(id int PRIMARY KEY,tax_id numeric REFERENCES parents(tax_id)); "
        "INSERT INTO parents VALUES(1,123451234.0); "
        "INSERT INTO items VALUES(1,123451234.00);",
        "",
        "SELECT tax_id FROM parents; SELECT tax_id FROM items; "
        "SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1;",
    ),
    "pattern_catalog": (
        "CREATE TABLE items(id int PRIMARY KEY,label text "
        "CHECK(label NOT LIKE '%pattern.canary@example.org%')); "
        "INSERT INTO items VALUES(1,'ok');",
        "",
        "SELECT pg_get_constraintdef(oid) FROM pg_constraint "
        "WHERE conrelid='items'::regclass;",
    ),
    "index_name": (
        "CREATE TABLE items(id int PRIMARY KEY,label text); "
        "CREATE INDEX secret_predicate ON items(id) WHERE label='Grace Hopper'; "
        "INSERT INTO items VALUES(1,'ok');",
        "",
        "SELECT indexdef FROM pg_indexes WHERE tablename='items';",
    ),
    "json_name_key": (
        "CREATE TABLE items(id int PRIMARY KEY,payload jsonb); "
        "INSERT INTO items VALUES(1,'{\"Grace Hopper\":\"ok\"}');",
        "",
        "SELECT payload FROM items;",
    ),
    "json_lowentropy_secret_key": (
        "CREATE TABLE items(id int PRIMARY KEY,payload jsonb); "
        "INSERT INTO items VALUES(1,'{\"patient-078-05-1120\":true}');",
        "",
        "SELECT payload FROM items;",
    ),
    "numeric_business": (
        "CREATE TABLE items(id int PRIMARY KEY,part_number bigint); "
        "INSERT INTO items SELECT i,100000000+i FROM generate_series(1,20)i;",
        "",
        "SELECT part_number FROM items ORDER BY id LIMIT 5;",
    ),
}


start = run(
    [
        "docker", "run", "--rm", "-d", "--name", name,
        "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-p", "127.0.0.1::5432",
        "postgres:18",
    ]
)
if start.returncode:
    raise RuntimeError(start.stderr)

results = []
try:
    for _ in range(100):
        if run(["docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"]).returncode == 0:
            break
        time.sleep(0.2)
    port = run(["docker", "port", name, "5432/tcp"]).stdout.strip().rsplit(":", 1)[1]
    for case, (source_sql, target_sql, query) in cases.items():
        source = "s_" + case
        target = "t_" + case
        sql("postgres", "CREATE DATABASE " + source)
        sql("postgres", "CREATE DATABASE " + target)
        setup = sql(source, source_sql, False)
        if setup["exit"]:
            result = {"case": case, "setup": setup}
            results.append(result)
            print(json.dumps(result), flush=True)
            continue
        if target_sql:
            sql(target, target_sql)
        dsn = "postgres://postgres@127.0.0.1:" + port + "/"
        proc = run(
            [
                binary, "--source", dsn + source + "?sslmode=disable",
                "--target", dsn + target + "?sslmode=disable",
                "--root", "public.items", "--take", "20", "--yes", "--no-config",
            ],
            cwd=root,
            env=dict(os.environ, LAZYSLICE_SECRET="42" * 32),
            timeout=90,
        )
        (root / (case + ".log")).write_text(proc.stdout + proc.stderr)
        result = {
            "case": case,
            "exit": proc.returncode,
            "target": sql(target, query, False),
            "log": str(root / (case + ".log")),
        }
        results.append(result)
        print(json.dumps(result), flush=True)
finally:
    (root / "results.json").write_text(json.dumps(results, indent=2))
    run(["docker", "rm", "-f", name])

print("Evidence directory:", root)
