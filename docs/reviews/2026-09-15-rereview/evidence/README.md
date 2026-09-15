# Re-review evidence

All probes were run against binary code from commit `c8ec5e2124d9f5fea2387a6028d152b80c2d8db3`, using disposable `postgres:18` containers. Values and credentials are synthetic.

Build a pinned binary and run the probes from the repository root:

```sh
git worktree add /tmp/lazyslice-review-c8ec5e2 c8ec5e2124d9f5fea2387a6028d152b80c2d8db3
go build -o /tmp/lazyslice-review-c8ec5e2-bin ./cmd/lazyslice
LAZYSLICE_REVIEW_BIN=/tmp/lazyslice-review-c8ec5e2-bin \
  python3 docs/reviews/2026-09-15-rereview/evidence/extended_probes.py
LAZYSLICE_REVIEW_BIN=/tmp/lazyslice-review-c8ec5e2-bin \
  python3 docs/reviews/2026-09-15-rereview/evidence/target_rls_race.py
LAZYSLICE_REVIEW_BIN=/tmp/lazyslice-review-c8ec5e2-bin \
  docs/reviews/2026-09-15-rereview/evidence/password_command_probe.sh
```

The exact observed summaries were:

```text
matview_target: exit=0; archive missing; items=1
citext_fk: exit=8; parent omitted; child=$lazyslice$invalid; marker=failed
date_fk: exit=12; target tables never created
numeric_fk: exit=8; parent=802938720; child=562764925; marker=failed
pattern_catalog: exit=0; CHECK ((label !~~ '%pattern.canary@example.org%'::text))
index_name: exit=12; target contains no indexes
json_name_key: exit=0; key "Grace Hopper" retained; leaf replaced
json_lowentropy_secret_key: exit=9; target table quarantined
numeric_business: exit=0; ordinary numeric values retained
RLS race: exit=0; inserted_after_gate=1; remaining_after_load=0; target_ids=1
password_command: exit=3; password authentication failed for source
```

The original race probe was also rerun without RLS. It exited 4, preserved the row inserted after the gate, and emitted `load.refused.target_changed`. The earlier review's remaining exact probes were rerun from `docs/reviews/2026-09-09/evidence`; their results are summarized in the parent review.

The materialized-view result checks `to_regclass('archive')` before selecting `items`; the blank first result means the view was dropped and the following `1` is the source row loaded into the replacement table.
