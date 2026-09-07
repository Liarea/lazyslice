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

## Decisions made during implementation

- **The catalogue is read through `event.Catalogue()`, and this package holds no
  copy of it.** `internal/event/catalogue.yml` is its home (ARCHITECTURE.md §12)
  and where a code is added; Go's `//go:embed` cannot reach outside a package
  directory, so this package once carried a byte-identical copy with
  `TestCatalogueIsTheEventCatalogue` comparing the two. `internal/event` now
  holds the `//go:embed` and exports the bytes (tracker T-0058): the copy, the
  second embed and that test are gone, and a row added to the catalogue is
  rendered here with nothing to keep in step.
- **`Lines` prints nothing for `StageStart` and `StageDone`.** They are the
  pipeline's own brackets and carry no fact the transcript needs — CONCEPT.md's
  transcript is a list of decisions and results — and `--json` still carries
  both. Skipping is a rendering decision, which is this package's job; the text
  of every line that *is* printed still comes from the catalogue.
- **A code with no catalogue row renders as a line naming the code.** That is a
  bug in the tree and not something a user did, and a renderer that printed
  nothing would silence a stage exactly where it had something to say.
- **A placeholder with no argument is left in the text.** A message with a hole
  in it names the missing argument; a message with a gap hides it.
- **`Event.Table`, `Event.Column` and `Event.Done` fill in for the `{table}`,
  `{column}` and `{count}` keys** when `Args` does not carry them, because they
  are typed fields and `Args` is a map of strings.
- Three tests here hold the catalogue itself: every row is complete, every error
  row carries an exit code, and no template references an argument key outside
  the `event.ArgKey` enum (THREAT_MODEL.md T4). The last one is the check
  ARCHITECTURE.md §7 asks for and it now exists.
