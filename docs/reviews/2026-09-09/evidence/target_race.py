import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

root=Path(tempfile.mkdtemp(prefix="lazyslice-review-race-"))
name="lazyslice-review-race-"+str(os.getpid())
def cmd(args, **kw):
    return subprocess.run(args,text=True,capture_output=True,**kw)
def sql(db,query):
    p=cmd(["docker","exec","-i",name,"psql","-XAt","-v","ON_ERROR_STOP=1","-U","postgres","-d",db],input=query)
    if p.returncode: raise RuntimeError(p.stderr)
    return p.stdout.strip()

p=cmd(["docker","run","--rm","-d","--name",name,"-e","POSTGRES_HOST_AUTH_METHOD=trust","-p","127.0.0.1::5432","postgres:18"])
if p.returncode: raise RuntimeError(p.stderr)
try:
    for _ in range(100):
        if cmd(["docker","exec",name,"pg_isready","-h","127.0.0.1","-U","postgres"]).returncode==0:break
        time.sleep(0.2)
    port=cmd(["docker","port",name,"5432/tcp"]).stdout.strip().rsplit(":",1)[1]
    sql("postgres","CREATE DATABASE s_race")
    sql("postgres","CREATE DATABASE t_race")
    sql("s_race","CREATE TABLE items(id integer PRIMARY KEY); INSERT INTO items VALUES (1)")
    sql("t_race","CREATE TABLE items(id integer PRIMARY KEY)")
    guard=subprocess.Popen(["docker","exec",name,"psql","-XAt","-U","postgres","-d","s_race","-c","SET application_name='review_guard'; BEGIN; LOCK TABLE items IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(25); COMMIT"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    for _ in range(100):
        if sql("s_race","SELECT count(*) FROM pg_stat_activity WHERE application_name='review_guard' AND wait_event='PgSleep'")=="1":break
        time.sleep(0.1)
    else:raise RuntimeError("guard lock not observed")
    dsn="postgres://postgres@127.0.0.1:"+port+"/"
    logfile=root/"race.log"
    with logfile.open("w") as out:
        child=subprocess.Popen(["/tmp/lazyslice-review","--source",dsn+"s_race?sslmode=disable","--target",dsn+"t_race?sslmode=disable","--root","public.items","--yes","--no-config"],cwd=root,env=dict(os.environ,LAZYSLICE_SECRET="42"*32),stdout=out,stderr=out)
        for _ in range(150):
            if "target t_race" in logfile.read_text():break
            if child.poll() is not None:raise RuntimeError(logfile.read_text())
            time.sleep(0.1)
        else:raise RuntimeError("target gate completion not observed")
        sql("t_race","INSERT INTO items VALUES (999)")
        before=sql("t_race","SELECT count(*) FROM items WHERE id=999")
        sql("s_race","SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='review_guard'")
        child.wait(timeout=60)
        guard.wait(timeout=5)
    after=sql("t_race","SELECT count(*) FROM items WHERE id=999")
    result={"exit":child.returncode,"inserted_after_gate":before,"remaining_after_load":after,"target_ids":sql("t_race","SELECT id FROM items ORDER BY id"),"log":str(logfile)}
    (root/"results.json").write_text(json.dumps(result,indent=2))
    print(json.dumps(result))
finally:
    cmd(["docker","rm","-f",name])
