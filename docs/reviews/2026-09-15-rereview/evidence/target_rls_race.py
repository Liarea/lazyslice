import json
import os
from pathlib import Path
import subprocess
import tempfile
import time


root = Path(tempfile.mkdtemp(prefix="lazyslice-rereview-rls-"))
name = "lazyslice-rereview-rls-" + str(os.getpid())
binary = os.environ.get("LAZYSLICE_REVIEW_BIN")
if not binary:
    raise RuntimeError("set LAZYSLICE_REVIEW_BIN to the pinned lazyslice binary")


def command(args, **kwargs):
    return subprocess.run(args, text=True, capture_output=True, **kwargs)


def sql(database, query):
    result = command(
        [
            "docker", "exec", "-i", name, "psql", "-XAt",
            "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", database,
        ],
        input=query,
    )
    if result.returncode:
        raise RuntimeError(result.stderr)
    return result.stdout.strip()


started = command(
    [
        "docker", "run", "--rm", "-d", "--name", name,
        "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-p", "127.0.0.1::5432",
        "postgres:18",
    ]
)
if started.returncode:
    raise RuntimeError(started.stderr)

try:
    for _ in range(100):
        if command(["docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"]).returncode == 0:
            break
        time.sleep(0.2)
    port = command(["docker", "port", name, "5432/tcp"]).stdout.strip().rsplit(":", 1)[1]
    sql("postgres", "CREATE ROLE reviewuser LOGIN")
    sql("postgres", "CREATE DATABASE s_race OWNER reviewuser")
    sql("postgres", "CREATE DATABASE t_race OWNER reviewuser")
    sql("s_race", "CREATE TABLE items(id integer PRIMARY KEY); ALTER TABLE items OWNER TO reviewuser; INSERT INTO items VALUES (1)")
    sql("t_race", "CREATE TABLE items(id integer PRIMARY KEY); ALTER TABLE items OWNER TO reviewuser")

    guard = subprocess.Popen(
        [
            "docker", "exec", name, "psql", "-XAt", "-U", "postgres", "-d", "s_race",
            "-c", "SET application_name='review_guard'; BEGIN; LOCK TABLE items IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(25); COMMIT",
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    for _ in range(100):
        if sql("s_race", "SELECT count(*) FROM pg_stat_activity WHERE application_name='review_guard' AND wait_event='PgSleep'") == "1":
            break
        time.sleep(0.1)
    else:
        raise RuntimeError("guard lock not observed")

    dsn = "postgres://reviewuser@127.0.0.1:" + port + "/"
    logfile = root / "race.log"
    with logfile.open("w") as output:
        child = subprocess.Popen(
            [
                binary, "--source", dsn + "s_race?sslmode=disable",
                "--target", dsn + "t_race?sslmode=disable",
                "--root", "public.items", "--yes", "--no-config",
            ],
            cwd=root,
            env=dict(os.environ, LAZYSLICE_SECRET="42" * 32),
            stdout=output,
            stderr=output,
        )
        for _ in range(150):
            if "target t_race" in logfile.read_text():
                break
            if child.poll() is not None:
                raise RuntimeError(logfile.read_text())
            time.sleep(0.1)
        else:
            raise RuntimeError("target gate completion not observed")

        sql(
            "t_race",
            "INSERT INTO items VALUES (999); "
            "ALTER TABLE items ENABLE ROW LEVEL SECURITY; "
            "ALTER TABLE items FORCE ROW LEVEL SECURITY",
        )
        before = sql("t_race", "SELECT count(*) FROM items WHERE id=999")
        sql("s_race", "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='review_guard'")
        child.wait(timeout=60)
        guard.wait(timeout=5)

    after = sql("t_race", "SELECT count(*) FROM items WHERE id=999")
    result = {
        "exit": child.returncode,
        "inserted_after_gate": before,
        "remaining_after_load": after,
        "target_ids": sql("t_race", "SELECT id FROM items ORDER BY id"),
        "log": str(logfile),
    }
    (root / "results.json").write_text(json.dumps(result, indent=2))
    print(json.dumps(result))
finally:
    command(["docker", "rm", "-f", name])
