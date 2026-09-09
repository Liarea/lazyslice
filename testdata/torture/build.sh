#!/usr/bin/env bash
#
# build.sh — rebuild one torture schema's schema.sql from its pinned upstream.
#
#     ./build.sh <name>        one schema
#     ./build.sh all           all ten
#
# `make torture` does not run this. The checked-in schema.sql files are the
# fixture; this script is how anyone checks that a file is what its README.md
# says it is, and how the pin is moved when someone decides to move it.
#
# Every schema ends the same way: the upstream artifact is turned into a real
# database in a throwaway container, and that database is dumped back with
# `pg_dump --schema-only --no-owner --no-privileges`. That is the whole reason
# the ten files look alike despite coming from a Rails structure.sql, a Rails
# schema.rb, 595 Prisma migrations, 75 Go-templated migrations, an Ecto dump, a
# Liquibase changelog, a Python ORM and a Ruby ORM: none of those is SQL anybody
# can load, and all of them make a database.
#
# The two `\restrict` / `\unrestrict` lines pg_dump has emitted since the August
# 2025 security releases are stripped, because they are psql meta-commands and
# the loader in internal/invariants is not psql (README.md says so as a rule).
#
# It needs Docker and network access. It is not run in CI.

set -euo pipefail

cd "$(dirname "$0")"
HERE="$PWD"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"; docker rm -f lstorture-build lstorture-vec lstorture-metabase >/dev/null 2>&1 || true' EXIT

PG_IMAGE="postgres:16@sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94"
PGVECTOR_IMAGE="pgvector/pgvector:pg16@sha256:ccc6e83d6e35e931dc7c5def2022729d5a6c370318d099181995567ff1fb4d6b"
ODOO_IMAGE="odoo:18@sha256:259fa933bf3ee7f3e375bd74d1e0bc28bd75955159723be477359e0fdb8acf67"
METABASE_IMAGE="metabase/metabase:v0.56.10@sha256:4b2bdce29288b8e94d73e44862644e84c940ab68e551ba9aefb1a8dc1738e9d8"
PYTHON_IMAGE="python:3.13-slim@sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285"
RUBY_IMAGE="ruby:3.3-slim@sha256:79f7a07363931fde1a5b312dee281fd62ddf56c33bdba0c2622af9b4621182fb"

DISCOURSE_COMMIT=7b13572c76fa2c32e86267c91c238b0a3d8084f9
GITLAB_COMMIT=f49b990568b44b24710efabf287200733f6fa930
MASTODON_COMMIT=26fce0f2f9e36ea0e0c2b03b7f57d1b1ea58ed1c
SUPABASE_COMMIT=0907af9bd6be3c76f472c40a7dcc0dc34abeffaf
PLAUSIBLE_COMMIT=e74d6fb214b76664442d6a6bb8d96e74807ccfbf
CALCOM_COMMIT=1251ba5be567d26a7f922452fe7797642376476e
DJANGO_VERSION=5.2.6
RAILS_VERSION=8.0.2

# The 21 GitLab tables the subset is seeded from. The closure is computed from
# them; gitlab/README.md explains the rule.
GITLAB_SEED="'users','namespaces','projects','issues','merge_requests','notes','members','milestones',
  'emails','user_details','user_preferences','personal_access_tokens','identities',
  'project_authorizations','issue_assignees','todos','events','abuse_reports','award_emoji',
  'project_settings','namespace_settings'"

say() { printf '==> %s\n' "$*"; }

# start_pg <container> <image> — a throwaway server, torn down by the trap.
start_pg() {
    docker rm -f "$1" >/dev/null 2>&1 || true
    docker run -d --name "$1" \
        -e POSTGRES_USER=lazyslice -e POSTGRES_PASSWORD=lazyslice -e POSTGRES_DB=postgres \
        -e POSTGRES_INITDB_ARGS=--nosync "$2" >/dev/null
    for _ in $(seq 1 60); do
        if docker exec "$1" pg_isready -U lazyslice -q 2>/dev/null; then return 0; fi
        sleep 1
    done
    echo "build.sh: $1 did not become ready" >&2
    exit 1
}

fresh_db() {
    docker exec "$1" psql -U lazyslice -d postgres -q \
        -c "DROP DATABASE IF EXISTS $2" -c "CREATE DATABASE $2" >/dev/null
}

psql_file() { docker exec -i "$1" psql -U lazyslice -d "$2" -q -v ON_ERROR_STOP=1 < "$3"; }

# dump <container> <db> <name> — the last step of every schema.
dump() {
    docker exec "$1" pg_dump -U lazyslice -d "$2" --schema-only --no-owner --no-privileges \
        | grep -v '^\\restrict\|^\\unrestrict' > "$HERE/$3/schema.sql"
    say "wrote $3/schema.sql ($(wc -l < "$HERE/$3/schema.sql" | tr -d ' ') lines)"
}

fetch() { curl -fsSL --retry 3 "$1" -o "$2"; }

# ---------------------------------------------------------------- discourse

build_discourse() {
    say "discourse: fetching db/structure.sql at $DISCOURSE_COMMIT"
    fetch "https://raw.githubusercontent.com/discourse/discourse/$DISCOURSE_COMMIT/db/structure.sql" \
        "$WORK/discourse.sql"
    start_pg lstorture-vec "$PGVECTOR_IMAGE"
    fresh_db lstorture-vec discourse
    psql_file lstorture-vec discourse "$WORK/discourse.sql"
    dump lstorture-vec discourse discourse
}

# ------------------------------------------------------------------- gitlab

build_gitlab() {
    say "gitlab: fetching db/structure.sql at $GITLAB_COMMIT"
    fetch "https://raw.githubusercontent.com/gitlabhq/gitlabhq/$GITLAB_COMMIT/db/structure.sql" \
        "$WORK/gitlab.sql"
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build gitlab
    psql_file lstorture-build gitlab "$WORK/gitlab.sql"

    # The keep set: the seed, closed over required parents and over the tables
    # the surviving triggers touch, to a fixed point.
    cat > "$WORK/keep.sql" <<SQL
DROP TABLE IF EXISTS lazyslice_keep;
CREATE TABLE lazyslice_keep(t text PRIMARY KEY);
INSERT INTO lazyslice_keep(t) VALUES ($(echo "$GITLAB_SEED" | sed "s/'\\([a-z_]*\\)'/('\\1')/g" | tr -d '\n'));
DO \$loop\$
DECLARE added integer; more integer;
BEGIN
  LOOP
    INSERT INTO lazyslice_keep(t)
    SELECT DISTINCT rc.relname::text
    FROM pg_constraint c
    JOIN pg_class tc ON tc.oid = c.conrelid JOIN pg_namespace tn ON tn.oid = tc.relnamespace
    JOIN pg_class rc ON rc.oid = c.confrelid JOIN pg_namespace rn ON rn.oid = rc.relnamespace
    WHERE c.contype = 'f' AND tn.nspname = 'public' AND rn.nspname = 'public'
      AND tc.relname::text IN (SELECT t FROM lazyslice_keep)
    ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS added = ROW_COUNT;

    INSERT INTO lazyslice_keep(t)
    SELECT DISTINCT other.relname::text
    FROM pg_trigger tg
    JOIN pg_class kc ON kc.oid = tg.tgrelid
    JOIN pg_namespace kn ON kn.oid = kc.relnamespace
    JOIN pg_proc p ON p.oid = tg.tgfoid
    JOIN pg_class other ON other.relkind IN ('r','p')
    JOIN pg_namespace on2 ON on2.oid = other.relnamespace AND on2.nspname = 'public'
    WHERE NOT tg.tgisinternal AND kn.nspname = 'public'
      AND kc.relname::text IN (SELECT t FROM lazyslice_keep)
      AND p.prosrc ~ ('\\m' || other.relname || '\\M')
    ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS more = ROW_COUNT;
    added := added + more;
    EXIT WHEN added = 0;
  END LOOP;
END
\$loop\$;
SQL
    docker exec lstorture-build psql -U lazyslice -d gitlab -q -c "DROP SCHEMA IF EXISTS gitlab_partitions_static CASCADE" >/dev/null
    docker exec lstorture-build psql -U lazyslice -d gitlab -q -c "DROP SCHEMA IF EXISTS gitlab_partitions_dynamic CASCADE" >/dev/null
    psql_file lstorture-build gitlab "$WORK/keep.sql"

    # One DROP per statement: a single transaction over 1,400 of them runs out
    # of max_locks_per_transaction.
    docker exec lstorture-build psql -U lazyslice -d gitlab -tAc "
      SELECT format('DROP TABLE IF EXISTS %I.%I CASCADE;', n.nspname, c.relname)
      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE c.relkind IN ('r','p')
        AND n.nspname NOT IN ('pg_catalog','information_schema')
        AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid = c.oid)
        AND NOT (n.nspname = 'public' AND c.relname IN (SELECT t FROM lazyslice_keep))
        AND c.relname <> 'lazyslice_keep'
      ORDER BY 1" > "$WORK/gitlab_drops.sql"
    docker exec -i lstorture-build psql -U lazyslice -d gitlab -q < "$WORK/gitlab_drops.sql" >/dev/null 2>&1 || true
    docker exec lstorture-build psql -U lazyslice -d gitlab -q -c "DROP TABLE lazyslice_keep" >/dev/null

    # The two objects that depend on a GitLab-defined function. §11.1 refuses a
    # recreated object that does, and gitlab/README.md records the deviation.
    docker exec lstorture-build psql -U lazyslice -d gitlab -tAc "
      SELECT format('DROP INDEX IF EXISTS %I.%I;', n.nspname, ic.relname)
      FROM pg_index i
      JOIN pg_class ic ON ic.oid = i.indexrelid
      JOIN pg_class c ON c.oid = i.indrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = 'public'
        AND EXISTS (SELECT 1 FROM pg_proc p JOIN pg_namespace pn ON pn.oid = p.pronamespace
                    WHERE pn.nspname = 'public'
                      AND pg_get_indexdef(i.indexrelid) ~ ('\\m' || p.proname || '\\M'))
      UNION ALL
      SELECT format('ALTER TABLE %I.%I ALTER COLUMN %I DROP DEFAULT;', n.nspname, c.relname, a.attname)
      FROM pg_attrdef d
      JOIN pg_class c ON c.oid = d.adrelid
      JOIN pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum
      JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = 'public'
        AND EXISTS (SELECT 1 FROM pg_proc p JOIN pg_namespace pn ON pn.oid = p.pronamespace
                    WHERE pn.nspname = 'public'
                      AND pg_get_expr(d.adbin, d.adrelid) ~ ('\\m' || p.proname || '\\M'))" \
      > "$WORK/gitlab_undepend.sql"
    say "gitlab: removing $(grep -c . "$WORK/gitlab_undepend.sql" || true) object(s) that depend on a GitLab function"
    docker exec -i lstorture-build psql -U lazyslice -d gitlab -q -v ON_ERROR_STOP=1 < "$WORK/gitlab_undepend.sql"

    dump lstorture-build gitlab gitlab
}

# ----------------------------------------------------------------- mastodon

build_mastodon() {
    say "mastodon: fetching db/schema.rb at $MASTODON_COMMIT"
    fetch "https://raw.githubusercontent.com/mastodon/mastodon/$MASTODON_COMMIT/db/schema.rb" "$WORK/schema.rb"
    cat > "$WORK/load_schema_rb.rb" <<'RUBY'
# Loads a Rails db/schema.rb into a live PostgreSQL database with nothing but
# ActiveRecord: no application, no migrations, no Rails boot.
require "tsort"
require "active_record"
require "scenic"
require "scenic/adapters/postgres"
require "scenic/statements"

Scenic.configure { |c| c.database = Scenic::Adapters::Postgres.new }
ActiveRecord::ConnectionAdapters::AbstractAdapter.include(Scenic::Statements)
ActiveRecord::Base.establish_connection(ENV.fetch("DATABASE_URL"))
ActiveRecord::Base.logger = nil
ActiveRecord::Migration.verbose = false
ActiveRecord::Schema.verbose = false
conn = ActiveRecord::Base.connection

# Mastodon's snowflake id default, from lib/mastodon/snowflake.rb. Upstream salts
# the hash with SecureRandom.hex(16), a different value on every install, which
# would make schema.sql different on every build; the salt is fixed here so the
# fixture is reproducible. Everything else is upstream's.
conn.execute(<<~SQL)
  CREATE OR REPLACE FUNCTION timestamp_id(table_name text)
  RETURNS bigint AS
  $$
    DECLARE
      time_part bigint;
      sequence_base bigint;
      tail bigint;
    BEGIN
      time_part := (((date_part('epoch', now()) * 1000))::bigint << 16);
      sequence_base := ('x' || substr(md5(table_name || '00000000000000000000000000000000' || time_part::text), 1, 4))::bit(16)::bigint;
      tail := ((sequence_base + nextval(table_name || '_id_seq')) & 65535);
      RETURN time_part | tail;
    END
  $$ LANGUAGE plpgsql VOLATILE;
SQL

load ARGV.fetch(0)

# The per-table sequences the default calls, as ensure_id_sequences_exist does.
conn.tables.each do |table|
  col = conn.columns(table).find { |c| c.name == "id" }
  next unless col
  m = /timestamp_id\('(?<seq_prefix>\w+)'/.match(col.default_function.to_s)
  next unless m
  conn.execute(%(DO $$ BEGIN CREATE SEQUENCE #{conn.quote_column_name("#{m[:seq_prefix]}_id_seq")}; EXCEPTION WHEN duplicate_table THEN END $$ LANGUAGE plpgsql;))
end
puts "loaded"
RUBY
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build mastodon
    docker run --rm --network "container:lstorture-build" -v "$WORK:/w" -w /w \
        -e DATABASE_URL=postgres://lazyslice:lazyslice@127.0.0.1:5432/mastodon "$RUBY_IMAGE" sh -c '
        set -e
        apt-get update -qq >/dev/null 2>&1
        apt-get install -y -qq --no-install-recommends build-essential libpq-dev >/dev/null 2>&1
        gem install --no-document activerecord -v "~> 8.1.0" >/dev/null
        gem install --no-document pg scenic >/dev/null
        ruby load_schema_rb.rb schema.rb'
    dump lstorture-build mastodon mastodon
}

# --------------------------------------------------------------------- odoo

build_odoo() {
    start_pg lstorture-build "$PG_IMAGE"
    docker exec lstorture-build psql -U lazyslice -d postgres -q -c "DROP DATABASE IF EXISTS odoo" >/dev/null
    say "odoo: initialising base,mail,contacts (this takes a few minutes)"
    docker run --rm --network "container:lstorture-build" \
        -e HOST=127.0.0.1 -e USER=lazyslice -e PASSWORD=lazyslice "$ODOO_IMAGE" \
        odoo -d odoo -i base,mail,contacts --without-demo=all --stop-after-init --log-level=warn
    dump lstorture-build odoo odoo
}

# ----------------------------------------------------------------- metabase

build_metabase() {
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build metabase
    say "metabase: starting the app so Liquibase migrates"
    docker rm -f lstorture-metabase >/dev/null 2>&1 || true
    docker run -d --name lstorture-metabase --network "container:lstorture-build" \
        -e MB_DB_TYPE=postgres -e MB_DB_DBNAME=metabase -e MB_DB_PORT=5432 \
        -e MB_DB_USER=lazyslice -e MB_DB_PASS=lazyslice -e MB_DB_HOST=127.0.0.1 \
        -e MB_JETTY_PORT=3999 "$METABASE_IMAGE" >/dev/null
    for _ in $(seq 1 120); do
        if docker logs lstorture-metabase 2>&1 | grep -q "Metabase Initialization COMPLETE"; then break; fi
        sleep 5
    done
    docker logs lstorture-metabase 2>&1 | grep -q "Metabase Initialization COMPLETE" \
        || { echo "build.sh: metabase never finished initialising" >&2; exit 1; }
    docker stop lstorture-metabase >/dev/null
    dump lstorture-build metabase metabase
}

# ------------------------------------------------------------ supabase-auth

build_supabase_auth() {
    say "supabase-auth: fetching migrations at $SUPABASE_COMMIT"
    mkdir -p "$WORK/sb"
    curl -fsSL "https://api.github.com/repos/supabase/auth/contents/migrations?ref=$SUPABASE_COMMIT" \
        | python3 -c 'import json,sys;[print(x["name"]) for x in json.load(sys.stdin)]' | sort > "$WORK/sb_names.txt"
    while read -r n; do
        fetch "https://raw.githubusercontent.com/supabase/auth/$SUPABASE_COMMIT/migrations/$n" "$WORK/sb/$n"
    done < "$WORK/sb_names.txt"

    : > "$WORK/sb_all.sql"
    printf 'CREATE SCHEMA IF NOT EXISTS auth;\n' >> "$WORK/sb_all.sql"
    while read -r n; do
        printf -- '\n-- ===== %s =====\n' "$n" >> "$WORK/sb_all.sql"
        # GoTrue's migrations are Go templates; the namespace a Supabase project
        # uses is `auth`. The trailing `;` is because two migrations end without
        # one and concatenation would run the next file into them.
        perl -pe 's/\{\{\s*index\s+\.Options\s+"Namespace"\s*\}\}/auth/g' "$WORK/sb/$n" >> "$WORK/sb_all.sql"
        printf '\n;\n' >> "$WORK/sb_all.sql"
    done < "$WORK/sb_names.txt"

    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build supabase
    # The roles the migrations grant to, with no attributes at all — never
    # superusers, and none of them survives a --no-owner --no-privileges dump.
    for r in postgres supabase_auth_admin authenticated anon service_role dashboard_user supabase_admin; do
        docker exec lstorture-build psql -U lazyslice -d supabase -q \
            -c "DO \$\$BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='$r') THEN CREATE ROLE $r; END IF; END\$\$;" >/dev/null
    done
    psql_file lstorture-build supabase "$WORK/sb_all.sql"
    dump lstorture-build supabase supabase-auth
}

# ------------------------------------------------------------------- calcom

build_calcom() {
    say "calcom: fetching prisma migrations at $CALCOM_COMMIT"
    mkdir -p "$WORK/cal"
    curl -fsSL "https://api.github.com/repos/calcom/cal.com/git/trees/$CALCOM_COMMIT?recursive=1" \
        | python3 -c '
import json,sys
d = json.load(sys.stdin)
if d.get("truncated"):
    sys.exit("build.sh: the cal.com tree came back truncated")
for x in d["tree"]:
    p = x["path"]
    if p.startswith("packages/prisma/migrations/") and p.endswith("migration.sql"):
        print(p)
' | sort > "$WORK/cal_paths.txt"
    while read -r p; do
        fetch "https://raw.githubusercontent.com/calcom/cal.com/$CALCOM_COMMIT/$p" \
            "$WORK/cal/$(basename "$(dirname "$p")").sql"
    done < "$WORK/cal_paths.txt"

    : > "$WORK/cal_all.sql"
    for f in $(ls "$WORK/cal"/*.sql | sort); do
        printf -- '\n-- ===== %s =====\n' "$(basename "$f" .sql)" >> "$WORK/cal_all.sql"
        cat "$f" >> "$WORK/cal_all.sql"
        printf '\n;\n' >> "$WORK/cal_all.sql"
    done

    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build calcom
    psql_file lstorture-build calcom "$WORK/cal_all.sql"
    dump lstorture-build calcom calcom
}

# ---------------------------------------------------------------- plausible

build_plausible() {
    say "plausible: fetching priv/repo/structure.sql at $PLAUSIBLE_COMMIT"
    fetch "https://raw.githubusercontent.com/plausible/analytics/$PLAUSIBLE_COMMIT/priv/repo/structure.sql" \
        "$WORK/plausible.sql"
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build plausible
    psql_file lstorture-build plausible "$WORK/plausible.sql"
    dump lstorture-build plausible plausible
}

# ------------------------------------------------------------------- django

build_django() {
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build django
    say "django: startproject + migrate on Django $DJANGO_VERSION"
    docker run --rm --network "container:lstorture-build" "$PYTHON_IMAGE" sh -c "
        set -e
        pip install --quiet --no-cache-dir 'django==$DJANGO_VERSION' 'psycopg[binary]'
        cd /tmp && django-admin startproject torture && cd torture
        cat >> torture/settings.py <<PY

DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.postgresql',
        'NAME': 'django',
        'USER': 'lazyslice',
        'PASSWORD': 'lazyslice',
        'HOST': '127.0.0.1',
        'PORT': '5432',
    }
}
PY
        python manage.py migrate --no-input >/dev/null"
    dump lstorture-build django django
}

# ------------------------------------------------------ rails-activestorage

build_rails_activestorage() {
    start_pg lstorture-build "$PG_IMAGE"
    fresh_db lstorture-build rails
    say "rails: rails new + active_storage:install + db:migrate on Rails $RAILS_VERSION"
    docker run --rm --network "container:lstorture-build" "$RUBY_IMAGE" sh -c "
        set -e
        apt-get update -qq >/dev/null 2>&1
        apt-get install -y -qq --no-install-recommends build-essential libpq-dev libyaml-dev git >/dev/null 2>&1
        gem install --no-document rails -v $RAILS_VERSION >/dev/null
        gem install --no-document pg >/dev/null
        cd /tmp
        rails new blog -d postgresql --skip-git --skip-test --skip-system-test --skip-kamal \
            --skip-solid --skip-bootsnap --skip-jbuilder --skip-brakeman --skip-ci --skip-rubocop \
            --skip-hotwire --skip-javascript --quiet
        cd blog
        export DATABASE_URL=postgres://lazyslice:lazyslice@127.0.0.1:5432/rails
        bin/rails active_storage:install >/dev/null
        bin/rails generate scaffold Post title:string body:text author_email:string >/dev/null
        bin/rails generate model Comment post:references author_name:string author_email:string body:text >/dev/null
        bin/rails db:migrate >/dev/null"
    dump lstorture-build rails rails-activestorage
}

# --------------------------------------------------------------------- main

ALL="discourse gitlab mastodon odoo metabase supabase-auth calcom plausible django rails-activestorage"

case "${1:-}" in
    all) targets="$ALL" ;;
    "")  echo "usage: $0 <name>|all   ($ALL)" >&2; exit 2 ;;
    *)   targets="$1" ;;
esac

for name in $targets; do
    case "$name" in
        discourse)           build_discourse ;;
        gitlab)              build_gitlab ;;
        mastodon)            build_mastodon ;;
        odoo)                build_odoo ;;
        metabase)            build_metabase ;;
        supabase-auth)       build_supabase_auth ;;
        calcom)              build_calcom ;;
        plausible)           build_plausible ;;
        django)              build_django ;;
        rails-activestorage) build_rails_activestorage ;;
        *) echo "build.sh: no such schema: $name" >&2; exit 2 ;;
    esac
done
