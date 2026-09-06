# internal/classify

Decides what every column holds: name/type/value signals, dictionaries,
scoring, and `reasons.go`'s reason templates. Pure — no SQL, ever; samples
arrive only through `pipeline.Sampler`, taken by `internal/introspect`. The
embedded rule pack (YAML) lives here.

**Contract.** ARCHITECTURE.md §4 and §2 "classify": `Classifier.Classify(schema,
Sampler, prior *Config) (*Classification, error)`.

**Rules.**
- Biased to recall, not precision: after the neighbouring-column rule and FK
  propagation, `possible` and above is masked, always — this is
  THREAT_MODEL.md T1's primary control; a change here that could lower recall
  needs a T1 review, not just a test pass.
- No exemption by type. An enum, a partition key, a `varchar(2)` — all
  classified like anything else (ARCHITECTURE.md §4; the earlier enum
  exemption was removed).
- Every `Decision.Reason` is rendered from the fixed template set in
  `reasons.go`; a template's placeholders admit only identifiers and counts.
  `TestReasonGrammar` parses every emitted reason back against the set — never
  add a template that could interpolate a sample value.
- The classifier is not pluggable (ADR-006): no runtime rule loading, no
  registration hook for a caller-supplied rule. A pattern in `lazyslice.yml`
  may only add a category or raise a confidence, never lower one
  (`TestConfigCannotLowerConfidence`).
- Every category that a column can be *masked* under has a registered masker
  in `mask`, and a test walks both lists (ADR-006 "Consequences"). The
  exception is the one category that names the absence of a signal: `CatNone`
  is what a copied column carries, it reaches no masker, and it is excluded
  from that walk by name rather than by a wildcard.

**Test.** `go test ./internal/classify/...` — no database needed, this package
is pure.

**Never:** issue SQL; let a reason string carry a sample value; add a type
exemption; add a way for a category's confidence to be lowered by config.
