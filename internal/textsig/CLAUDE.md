# internal/textsig

The value-only half of ARCHITECTURE.md §4's validators, plus the embedded
English name dictionary: a pure function over one string, and nothing else.
`internal/classify` scores its signals with these and `internal/verify`'s second
net re-runs them over the loaded target, so this is the one home for both
(tracker T-0055).

**Contract.** ARCHITECTURE.md §4 "Signals", the value half only. A validator is
`func(string) bool` — total, allocation-light, no context, no error. The
dictionary is `Dictionary() *Dict`, parsed once from the embedded `names.txt`
and shared; its set is unexported, so no caller can add a word to a dictionary
every other caller reads.

**Rules.**
- It is a leaf. It imports the standard library and
  `github.com/nyaruka/phonenumbers` and nothing from this repository at all —
  not `ref`, not `pipeline`. internal/CLAUDE.md's import graph allows a leaf
  under `internal/` that imports only `ref` and `pipeline` (that is what `ref`
  and `event` already are); this one is stricter than the rule permits, and it
  should stay that way. Nothing here needs a `ColumnRef` or a `Category`,
  because nothing here knows what a column is.
- **No rule pack lives here.** No name pattern, no category, no confidence, no
  threshold, no scoring. Those are `internal/classify`'s, they are not
  duplicated, and a validator here must never grow a second argument that
  smuggles one in. The dictionary is a word list, which is why it can be here at
  all; the *scoring* over it is the caller's, and the two callers score it
  differently on purpose (see below).
- A validator answers about one value and never about a column. Ratios,
  thresholds and minimum counts belong to the caller.
- A change here changes both callers, which is the point — and it changes what
  `internal/classify` masks, so it is a THREAT_MODEL.md T1 question and not a
  refactor.

**The two dictionary signals, and why there are four validators.** Each signal
has a wide form the classifier scores and a narrow form the second net scores,
because a `true` costs the two callers different things: `internal/classify`
masks a column, which costs a lookup table, and `internal/verify` refuses a
target that is already loaded, at exit 9, with `--unmask` the only way past it.

- **A name.** `Dict.LooksLikeName` is the classifier's: one to three dictionary
  words and nothing else, so a `first_name` column of one word per row is
  caught. `Dict.NameShape` is `internal/verify`'s: two or three dictionary
  words *and* a given name immediately followed by a surname. The middle form —
  "two or three dictionary words", with no given/surname pair — was tried first
  and is not enough, because the surname section holds about two hundred
  ordinary English words: `green lane`, `west hill` and `hunter green` are all
  pairs of dictionary surnames (tracker T-0055's review). That is why `names.txt`'s
  two sections are kept apart in `Dict` rather than merged into one set.
- **Prose.** `Dict.Prose` is the classifier's: six words or more with a
  dictionary word inside (`Dict.ContainsName`). Its word floor excludes short
  values and **nothing more** — an earlier version of this file and of
  `dict.go` claimed the floor also kept a single dictionary word out, and that
  was wrong: "The supplier may terminate this agreement on thirty days notice."
  is prose with a name in it, because *may* is a surname. `Dict.ProseName` is
  `internal/verify`'s: the same word floor with the `NameShape` pair required
  somewhere inside the string, a sentence boundary breaking the adjacency.

The rule verify scores them under is named in internal/verify/CLAUDE.md, "the
dictionary rule". Both narrow forms fail by *missing* a name — a name written
surname-first, a name the dictionary does not carry — which is the direction a
refusal after the target is loaded has to fail in, and neither narrows what the
classifier masks.

**Test.** `go test ./internal/textsig/...` — this package has no test of its
own. Its behaviour is pinned where the decisions are made: the classifier's
precision and recall suite (`TestPagilaPrecisionAndRecall`,
`TestFiftyNamesFromThreeSchemas` in `internal/classify`) and the second net's
thresholds in `internal/verify`. A validator changed here that breaks either is
supposed to break it.

**Never:** read a column name, a neighbour, a schema or a database; return
anything but a verdict about the string you were handed; export the dictionary's
set; add a category, a confidence or a threshold to this package.

## Owed elsewhere

- `internal/CLAUDE.md`'s import-graph rule and ARCHITECTURE.md §2's "Import
  graph" both name the packages one by one and do not yet name this one; §12's
  layout does not list it either. Neither file is in T-0055's paths — **tracker
  T-0086** carries the edit.
- The `textOf`/`mask.Canonical` reproduction, the type-family table and the
  identifier quoter are the *other* copies internal/verify/CLAUDE.md owes a
  shared home. T-0055 moved the validators and the dictionary only. If they land
  here later, they arrive with the same rule as above: a value shape, never a
  rule pack — and `mask.Canonical` reproduction may not live here at all if it
  needs `pipeline`, because this package's import list is a feature.
