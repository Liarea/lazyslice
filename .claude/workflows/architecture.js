export const meta = {
  name: 'lazyslice-architecture',
  description: 'Phase 2: benchmark, three independent architecture proposals, judge panel, ADRs, threat model, adversarial review',
  phases: [
    { title: 'Propose', detail: 'three architects with different lenses, plus a language throughput benchmark' },
    { title: 'Judge', detail: 'three judges score every proposal' },
    { title: 'Decide', detail: 'Fable writes ADRs 001-006, ARCHITECTURE.md, THREAT_MODEL.md' },
    { title: 'Review', detail: 'three adversarial lenses on the decision set, one revision' },
  ],
}

const REPO = '/Users/gareth/personal_repos/lazyslice'
const PRE = `You are on the lazyslice team. Repo: ${REPO}. Read ${REPO}/CONCEPT.md, ${REPO}/CLAUDE.md, ${REPO}/research/SYNTHESIS.md, and ${REPO}/research/HARD_PROBLEMS.md first; consult the other files in ${REPO}/research/ as needed.
Rules: write ONLY the file(s) your task names, using absolute paths. Do not edit any other file. Do not run git commit. Cite research documents by path and heading when you rely on them. Do not stop until the file is written and complete; nobody is watching and nobody can answer questions. Your final message is not for a human: return only the structured output.`

const OUT = { type: 'object', required: ['files', 'summary', 'postmortem'], properties: {
  files: { type: 'array', items: { type: 'string' } }, summary: { type: 'string', description: 'At most 150 words' }, postmortem: { type: 'string' } } }

const LENSES = [
  { key: 'mvp-first', brief: 'Optimise for the shortest path to a working Postgres-to-Postgres vertical slice that a stranger can run in one command. Cut anything that does not serve gate 4 in docs/BUILD_PLAN.md.' },
  { key: 'risk-first', brief: 'Optimise for never leaking production data and never touching the source. Every design choice is justified by the threat it removes. Assume the tool will be run by tired people against real prod.' },
  { key: 'user-first', brief: 'Optimise for the first-run delight described in research/SQLIT_STUDY.md and the complaints in research/COMPLAINTS.md. Every choice is justified by a user quote or a sqlit or lazygit mechanism.' },
]

const DECISIONS = `The six decisions every proposal must take a position on, with reasoning and a reversal condition: (1) implementation language and why, given research/BENCHMARK_LANGUAGE.md if it exists yet; (2) TUI framework; (3) database order and what "done" means for an adapter; (4) configuration model: emitted after a run versus required before; (5) the pipeline: stage names, the interface each exposes as types, how a headless CLI and a TUI share the core, how progress events flow; (6) extension model: which of classifier and maskers are pluggable and by what mechanism. Also specify the subset planner algorithm concretely (root, N, parents to completeness, children with caps, cycle handling, unreachable lookup tables), the deterministic masking scheme, and the v1 safety controls.`

log('Phase 2: benchmark plus three proposals in parallel')

const [bench, proposals] = await parallel([
  () => agent(`${PRE}
Settle the language throughput question with a measurement, not an opinion. Start a PostgreSQL 16 container with Docker (name lazyslice-bench, a random high port, remove it when done). Create a table with 5 million rows and ten mixed columns (ints, text, timestamps, a JSONB). Measure wall-clock and peak RSS for: (a) Go with pgx v5 using CopyTo into a streaming consumer that counts rows and writes them back with CopyFrom into a second table; (b) Python 3.13 with psycopg 3 doing the same with copy() in both directions; (c) the psql \\copy baseline. Run each three times. Put the code under /private/tmp/claude-501/lazyslice-scratch/bench/ (not in the repo). Write ${REPO}/research/BENCHMARK_LANGUAGE.md with the table of results, the exact commands, hardware, and a one-paragraph interpretation that says which language wins for streaming and by how much, and whether the gap matters at the scale in CONCEPT.md. Stop and remove the container at the end even if something fails.`,
    { label: 'benchmark', phase: 'Propose', model: 'opus', schema: OUT }),
  () => parallel(LENSES.map(l => () => agent(`${PRE}
You are one of three architects writing independent proposals. Your lens: ${l.brief}
Write ${REPO}/research/proposals/${l.key}.md (create the directory if needed). ${DECISIONS}
Include a Mermaid diagram of the pipeline and one of the FK walk on a schema with a cycle. Be concrete: name libraries with versions, name types, name flags. Under 2000 words.`,
    { label: `propose:${l.key}`, phase: 'Propose', model: l.key === 'risk-first' ? 'fable' : 'opus', schema: OUT }))),
])

const JUDGE = { type: 'object', required: ['scores', 'best_ideas', 'fatal_flaws'], properties: {
  scores: { type: 'array', items: { type: 'object', required: ['proposal', 'total', 'notes'], properties: { proposal: { type: 'string' }, total: { type: 'integer' }, notes: { type: 'string' } } } },
  best_ideas: { type: 'array', items: { type: 'string' }, description: 'ideas worth keeping from proposals that did not win' },
  fatal_flaws: { type: 'array', items: { type: 'string' } } } }

log('Judging: three judges, each scores all three proposals')
const judges = await parallel(['implementability by one developer in eight weeks', 'data-safety and failure modes', 'first-run experience and adoption'].map((crit, i) => () => agent(`${PRE}
You are a judge. Read all three files in ${REPO}/research/proposals/ and ${REPO}/research/BENCHMARK_LANGUAGE.md. Your primary criterion is: ${crit}. Score each proposal 0-10 on your criterion and 0-10 on each of: internal consistency, fidelity to CONCEPT.md principles, and evidence cited from research/. Total out of 40. Name the best idea in each proposal that the others lack, and any fatal flaw. Be specific and quote the proposals.`,
  { label: `judge:${i + 1}`, phase: 'Judge', model: 'opus', schema: JUDGE })))

const decide = await agent(`${PRE}
You are the architect making the final call. Inputs: the three proposals in ${REPO}/research/proposals/, ${REPO}/research/BENCHMARK_LANGUAGE.md, and the judges' verdicts: ${JSON.stringify(judges.filter(Boolean))}.
Write these files: ${REPO}/docs/adr/001-language.md, 002-tui.md, 003-database-order.md, 004-config-model.md, 005-pipeline.md, 006-extension-model.md, each in the format Context / Options considered / Decision / Consequences / Reversal condition, citing proposals and research by path. Then ${REPO}/ARCHITECTURE.md: the pipeline stages as Go interfaces with their input and output types written as real Go code, the subset planner algorithm as pseudocode, the deterministic masking scheme, the progress event model, the CLI flag surface for v1 (every flag, its default, and which stage it drives), how the emitted lazyslice.yml looks with a full example, the repository layout with one line per directory, the dependency list with versions and one reason each, and two Mermaid diagrams. Then ${REPO}/THREAT_MODEL.md: assets, threats (at minimum: classifier misses a PII column, target is production, secrets in config or logs, snapshot committed to git, malicious custom masker, half-loaded target, supply chain of releases), each with likelihood, impact, the concrete control, and whether it blocks v1. Finally ${REPO}/docs/adr/README.md explaining the format and listing the ADRs. Graft the judges' best ideas where they fit. Where the judges disagree, decide and say why.`,
  { label: 'decide', phase: 'Decide', model: 'fable', effort: 'high', schema: OUT })

const REV = { type: 'object', required: ['verdict', 'issues'], properties: { verdict: { type: 'string', enum: ['accept', 'revise'] },
  issues: { type: 'array', items: { type: 'object', required: ['severity', 'file', 'issue', 'fix'], properties: { severity: { type: 'string', enum: ['high', 'medium', 'low'] }, file: { type: 'string' }, issue: { type: 'string' }, fix: { type: 'string' } } } } } }

log('Adversarial review of the decision set')
const reviews = await parallel([
  'You are a senior Go engineer. Can one developer implement ARCHITECTURE.md as written? Are the interfaces implementable with the named libraries at the named versions (verify on pkg.go.dev)? Is anything under-specified such that two developers would build incompatible stages? Does the planner pseudocode actually terminate on cycles?',
  'You are a security reviewer who will be blamed if this tool leaks production data. Attack THREAT_MODEL.md and the masking scheme in ARCHITECTURE.md. Find a threat not listed, a control that does not actually control, and a way the determinism scheme could let someone correlate masked values back to real ones.',
  'You are a developer who has never seen this tool, reading ARCHITECTURE.md and the ADRs. Would the first run described be zero-config in practice? Does the flag surface contradict the config model? Does anything require documentation to use? Compare against research/SQLIT_STUDY.md and research/COMPLAINTS.md.',
].map((lens, i) => () => agent(`${PRE}\nRead every file in ${REPO}/docs/adr/, ${REPO}/ARCHITECTURE.md, and ${REPO}/THREAT_MODEL.md. ${lens} Return revise if any high-severity issue exists.`,
  { label: `review:${i + 1}`, phase: 'Review', model: 'opus', schema: REV })))

const issues = reviews.filter(Boolean).flatMap(r => r.issues).filter(i => i.severity !== 'low')
let revision = null
if (issues.length) {
  log(`${issues.length} high or medium issues; revising once`)
  revision = await agent(`${PRE}\nYou wrote the ADRs, ARCHITECTURE.md, and THREAT_MODEL.md. Reviewers found: ${JSON.stringify(issues)}. Address every item with targeted edits to the named files. Where you disagree with a reviewer, add a short "Review note" in the file saying why. Return the list of files changed.`,
    { label: 'revise', phase: 'Review', model: 'fable', effort: 'high', schema: OUT })
}

return { bench, proposals: proposals.filter(Boolean), judges: judges.filter(Boolean), decide, issues, revision }
