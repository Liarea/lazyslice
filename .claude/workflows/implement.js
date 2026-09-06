export const meta = {
  name: 'lazyslice-implement',
  description: 'One task: developer implements, three reviewers, up to two fix rounds, independent verify, commit',
  phases: [
    { title: 'Build', detail: 'developer implements the brief and runs the checks' },
    { title: 'Review', detail: 'correctness, safety, scope reviewers in parallel' },
    { title: 'Fix', detail: 'developer addresses findings, reviewer re-verifies' },
    { title: 'Verify', detail: 'independent check run and commit' },
  ],
}
// args: { id, title, brief, model, effort, paths, stage }
//   id     tracker task id (for the commit message)
//   title  short title
//   brief  the full task text (from docs/BUILD_PLAN.md or written by the orchestrator)
//   model  developer model: 'opus' | 'sonnet'
//   paths  directories or files the developer may write, as an array of repo-relative paths
//   stage  pipeline stage name or 'peripheral'
//   reviewers  1 or 3 (default 3); peripheral docs tasks use 1
//   integration  true: run make integration in verify (only once the pipeline can pass I1-I6); a package pattern string like './internal/pg/...': run that package's integration-tagged tests instead
//   checks  'full' (default) or 'none' for documentation-only tasks; parallel code tasks are verified by the orchestrator after the batch

const REPO = '/Users/gareth/personal_repos/lazyslice'
const a = args || {}
if (!a.brief || !a.id) throw new Error('implement.js needs args {id, title, brief, model, paths}')
const model = a.model || 'opus'
const effort = a.effort || 'high'
const paths = (a.paths || []).join(', ')
const nReviewers = a.reviewers === 1 ? 1 : 3

const PRE = `You are a developer on lazyslice. Repo: ${REPO}. Read ${REPO}/CLAUDE.md, ${REPO}/ARCHITECTURE.md, and the CLAUDE.md in every directory you touch, before writing code.
Rules: you may write only under these paths: ${paths}. You may not edit go.mod or go.sum; if you need a dependency that is missing, stop and report it in your return value. Do not fix nearby code, do not extend behaviour the task did not mention, do not add tests beyond the task. If you must deviate from ARCHITECTURE.md (a module path, a signature, a default), say so in concerns; never silently. Prefer targeted edits over rewriting files. Run gofmt on files you touch. Before returning, run: make lint && make test (and make integration if your task says so), and paste the last 20 lines of output into your return value. "Should work" is not a status. Do not commit. Nobody is watching and nobody can answer questions: finish the whole task. A previous attempt at this task may have been interrupted: if files under your paths already exist, read them and continue from that state rather than starting over.`

const DEV = { type: 'object', required: ['files', 'summary', 'checks_output', 'checks_passed', 'concerns', 'postmortem'], properties: {
  files: { type: 'array', items: { type: 'string' } }, summary: { type: 'string', description: 'at most 150 words' },
  checks_output: { type: 'string', description: 'last 20 lines of make lint && make test' }, checks_passed: { type: 'boolean' },
  concerns: { type: 'array', items: { type: 'string' }, description: 'things noticed outside scope, missing deps, doubts' },
  postmortem: { type: 'string', description: 'went well | went badly | change next time' } } }

const FINDINGS = { type: 'object', required: ['findings'], properties: { findings: { type: 'array', items: { type: 'object',
  required: ['severity', 'file', 'line', 'issue', 'fix'], properties: { severity: { type: 'string', enum: ['high', 'medium', 'low'] },
  file: { type: 'string' }, line: { type: 'integer' }, issue: { type: 'string' }, fix: { type: 'string' } } } } } }

const LENSES = [
  { key: 'correctness', text: 'Correctness. Read the diff (git diff) and the files listed. Find bugs: wrong logic, unhandled errors, off-by-one, races, resource leaks, Postgres edge cases (nulls, composite keys, quoted identifiers, schemas other than public, partitions). Does every test actually assert the behaviour the brief asks for, or could a stub pass it?' },
  { key: 'safety', text: 'Data safety and invariants. Could this change let unmasked personal data reach the target, write to the source, log a secret, or leave a half-loaded target? Does it weaken any invariant I1-I6 in ARCHITECTURE.md or a control in THREAT_MODEL.md? Is any flag added that reduces masking?' },
  { key: 'scope', text: 'Scope and simplicity. Did the developer do only what the brief asked? Flag unrequested changes, extra files, speculative abstractions, dependencies added, and anything that belongs to a later phase per ROADMAP.md. Flag code that two other developers would each have to understand to work on the next stage.' },
]

log(`Task ${a.id}: ${a.title} (developer: ${model})`)
let dev = await agent(`${PRE}\n\n<task id="${a.id}">\n${a.brief}\n</task>`, { label: `build:${a.id}`, phase: 'Build', model, effort, schema: DEV })
if (!dev) return { id: a.id, status: 'blocked', reason: 'developer agent died', findings: [] }

const review = async (round) => {
  const rs = await parallel(LENSES.slice(0, nReviewers).map(l => () => agent(`You are a reviewer on lazyslice. Repo: ${REPO}. Read ${REPO}/CLAUDE.md and ${REPO}/ARCHITECTURE.md. The task under review is:\n<task id="${a.id}">\n${a.brief}\n</task>\nThe developer reports: ${dev.summary}. Files: ${dev.files.join(', ')}. Checks passed: ${dev.checks_passed}.\nYour lens: ${l.text}\nRun the checks yourself (make lint && make test) and do not trust the developer's report. Do not fix anything. Return findings with file and line; severity high means it must not merge.`,
    { label: `review:${l.key}:r${round}`, phase: 'Review', model: 'opus', schema: FINDINGS })))
  return rs.filter(Boolean).flatMap(r => r.findings)
}

let findings = await review(1)
let blocking = findings.filter(f => f.severity !== 'low')
let round = 0
while (blocking.length && round < 2) {
  round++
  log(`Round ${round}: ${blocking.length} findings to fix`)
  const fixed = await agent(`${PRE}\n\nYou implemented task ${a.id}: ${a.title}. Reviewers found:\n${JSON.stringify(blocking, null, 1)}\nAddress every high and medium finding with targeted edits. If you believe a finding is wrong, say so in concerns with your reasoning rather than silently ignoring it. Re-run the checks.`,
    { label: `fix:${a.id}:r${round}`, phase: 'Fix', model, effort, schema: DEV })
  if (!fixed) break
  dev = fixed
  const re = await agent(`You are the re-verifier on lazyslice. Repo: ${REPO}. These findings were reported on task ${a.id} and the developer says they are fixed:\n${JSON.stringify(blocking, null, 1)}\nDeveloper's concerns: ${JSON.stringify(dev.concerns)}. Check each finding against the current code (git diff, read the files). Run make lint && make test. Return only findings that are still open, keeping their severity, plus any new high-severity problem the fix introduced.`,
    { label: `reverify:${a.id}:r${round}`, phase: 'Fix', model: 'opus', schema: FINDINGS })
  blocking = re ? re.findings.filter(f => f.severity !== 'low') : []
}

if (blocking.length) {
  log(`Task ${a.id} blocked after ${round} fix rounds`)
  return { id: a.id, status: 'blocked', findings: blocking, dev, postmortem: dev.postmortem }
}

const integ = a.integration === true ? ' && make integration' : (typeof a.integration === 'string' ? ` && go test -tags integration -count=1 ${a.integration}` : '')
const verify = a.checks === 'none' ? { passed: true, output: 'checks skipped: documentation task' } : await agent(`Repo: ${REPO}. Run exactly: cd ${REPO} && make lint && make test${integ}. Report whether every command exited 0 and paste the last 30 lines of output. Do not change any file.`,
  { label: `verify:${a.id}`, phase: 'Verify', model: 'sonnet', effort: 'low', schema: { type: 'object', required: ['passed', 'output'], properties: { passed: { type: 'boolean' }, output: { type: 'string' } } } })

if (!verify || !verify.passed) {
  return { id: a.id, status: 'blocked', findings: [{ severity: 'high', file: 'Makefile', line: 0, issue: 'independent check run failed', fix: (verify && verify.output) || 'no output' }], dev, postmortem: dev.postmortem }
}

const commit = await agent(`Repo: ${REPO}. Run: cd ${REPO} && git add -A && git commit -m "${(a.stage || 'peripheral')}: ${a.title} (${a.id})" and report the short hash. If there is nothing to commit, say so. Do not change any file.`,
  { label: `commit:${a.id}`, phase: 'Verify', model: 'haiku', effort: 'low', schema: { type: 'object', required: ['hash'], properties: { hash: { type: 'string' } } } })

return { id: a.id, status: 'merged', hash: commit ? commit.hash : '', files: dev.files, summary: dev.summary, concerns: dev.concerns, findings_low: findings.filter(f => f.severity === 'low'), rounds: round, postmortem: dev.postmortem }
