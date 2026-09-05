# internal/render

The two v1 event sinks: `Lines` (default human transcript) and `NDJSON`
(`--json`). Formats events into output; never reaches a stage and never
decides what happened, only how it's shown.

**Contract.** ARCHITECTURE.md §7: consumes `event.Event` off the bounded
channel `core.Run` sends into. `Lines` renders each `Code` from
`internal/event/catalogue.yml`, the same source `docs/ERRORS.md` is generated
from.

**Rules.**
- Every line of human-readable text comes from the catalogue, not a literal
  string composed here — text a user sees and text `docs/ERRORS.md` promises
  must never drift apart (CI checks this).
- `NDJSON` writes events verbatim, one JSON object per line — it must never
  reformat, summarize, or drop a field `event.Event` carries.
- Colour/style is a property of the renderer (`Lines.Style`), never of a
  stage — a stage must never be able to change what colour its own event
  prints in.
- A `PgError` is rendered elsewhere (`internal/pg.RenderError`) before it
  reaches a sink — this package does not know about `Detail`/`Where`/`Hint`
  redaction; it renders whatever `Code`+`Args` it's given.

**Test.** `go test ./internal/render/...`, including a golden-output test per
`Code` once the catalogue exists.

**Never:** format a message inline instead of through the catalogue; drop or
reorder fields in `NDJSON` output; let a renderer reach a stage or a database
connection.
