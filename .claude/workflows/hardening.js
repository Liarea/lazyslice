export const meta = {
  name: 'lazyslice-hardening',
  description: 'Phase 5: torture schemas, failure UX, performance, then a three-attacker red team with one fix round and re-attack',
  phases: [
    { title: 'Harden', detail: 'torture, failure UX, performance, each through implement.js in sequence' },
    { title: 'Red team', detail: 'three attackers try to make the tool leak; fix; re-attack' },
  ],
}
const REPO = '/Users/gareth/personal_repos/lazyslice'
const IMPL = `${REPO}/.claude/workflows/implement.js`

const TASKS = [
  { id: 'T-TORTURE', title: 'Schema torture suite', stage: 'hardening', paths: ['testdata/torture/', 'internal/', 'Makefile', 'docs/TORTURE.md'], integration: true,
    brief: `Collect ten real open-source PostgreSQL schemas of different shapes: Discourse, GitLab (a trimmed subset is acceptable, say why), Mastodon, Odoo (subset), Metabase, Supabase auth schema, Cal.com, Plausible, a Django default project with auth and admin, and a Rails default with ActiveStorage. Put each under testdata/torture/<name>/ with its source URL and commit pinned in a README. Add a make torture target that loads each into a container with a small amount of generated data (a generator script per schema, a few hundred rows in the most-connected tables) and runs a snapshot from its most-connected table into a second container, then the invariant checks. For every failure, write a minimal reproduction into testdata/regressions/ before fixing it in internal/. Write docs/TORTURE.md with a table: schema, tables, run time, failures found and fixed, and for three schemas a hand-labelled PII truth set with precision and recall. Do not stop until at least nine of ten snapshot cleanly and the tenth fails with a message that names the exact cause.` },
  { id: 'T-FAILUX', title: 'Failure UX and error catalogue', stage: 'hardening', paths: ['internal/', 'cmd/', 'docs/ERRORS.md'], integration: true,
    brief: `Audit every error path. Each user-facing error must say what went wrong, which table or column, and the exact flag or action that fixes it. No stack trace reaches the user without --debug. Partial failures must leave the target either empty or complete; verify the drop-on-failure path with a test that kills the load mid-way. Define an error type per failure class in one package and write docs/ERRORS.md as the catalogue; add a test that every error type in the code appears in the catalogue and vice versa. Every fix is a targeted edit; do not restructure packages.` },
  { id: 'T-PERF', title: 'Performance baseline', stage: 'hardening', paths: ['internal/extract/', 'internal/load/', 'internal/transform/', 'docs/PERF.md', 'Makefile', '.github/'], integration: true,
    brief: `Profile a snapshot of 5,000 root rows from a source whose child table has 20,000,000 rows (extend the nasty.sql generator with a size parameter). Report where time goes per stage with pprof. Target: under 3 minutes on this machine against local containers. Optimise only the top two hotspots. Write the before and after numbers, the commands, and the flame summary into docs/PERF.md. Add a make bench target and a CI job that fails if extract throughput drops more than 20% from the recorded baseline stored in testdata/bench/baseline.json.` },
]

phase('Harden')
const results = []
for (const t of TASKS) {
  const r = await workflow({ scriptPath: IMPL }, { ...t, model: 'opus', effort: 'high' })
  results.push(r)
  if (!r || r.status !== 'merged') { log(`Stopped at ${t.id}`); return { results, stopped_at: t.id } }
}

phase('Red team')
const ATTACK = { type: 'object', required: ['attempts'], properties: { attempts: { type: 'array', items: { type: 'object', required: ['attack', 'reproduction', 'leaked', 'fix'], properties: { attack: { type: 'string' }, reproduction: { type: 'string' }, leaked: { type: 'boolean' }, fix: { type: 'string' } } } } } }
const attackers = [
  'PII that dodges the classifier: column names in other languages or abbreviations, values that miss every regex (obfuscated emails, phone numbers with spaces and words, names in a column called label), personal data inside arrays and JSON three levels deep, and inside a column the classifier marked as safe. Build a schema for each, run the tool, and check the target.',
  'Wrong target: make the tool write to production by aliasing the host, using a different port on the same database, a read replica, a connection string with the same database name on a different host, or a target URL taken from a compose file that points at prod. Also try to get it to hold write privileges on the source.',
  'Secrets and residue: get a password, secret, or unmasked row into lazyslice.yml, logs at any verbosity, the emitted error messages, a crash dump, a temp file, or a snapshot written into a git-tracked or cloud-synced folder. Also try a custom masker that throws halfway and one that returns its input.',
]
const attack = (round) => parallel(attackers.map((a, i) => () => agent(`You are a red-team engineer trying to make lazyslice leak production data. Repo: ${REPO}. Read THREAT_MODEL.md, ARCHITECTURE.md, and the code. Build the binary. Use the testutil containers. Your attack surface: ${a} For each attempt give the exact reproduction commands, whether anything leaked or the tool misbehaved, and the fix. Do not change any repo file; put scratch schemas under /private/tmp/claude-501/lazyslice-scratch/redteam-${i}/. Report every attempt, including the ones that failed to leak.`,
  { label: `attack:${i + 1}:r${round}`, phase: 'Red team', model: 'opus', schema: ATTACK })))

const first = (await attack(1)).filter(Boolean).flatMap(r => r.attempts)
const leaks = first.filter(x => x.leaked)
log(`Red team round 1: ${first.length} attempts, ${leaks.length} leaked`)
let fix = null, second = []
if (leaks.length) {
  fix = await workflow({ scriptPath: IMPL }, { id: 'T-REDFIX', title: 'Red team fixes', model: 'opus', effort: 'high', stage: 'hardening', paths: ['internal/', 'cmd/', 'THREAT_MODEL.md', 'testdata/regressions/'], integration: true,
    brief: `The red team made lazyslice leak or misbehave in these ways:\n${JSON.stringify(leaks, null, 1)}\nFor each: add a regression test under testdata/regressions/ or the relevant package that reproduces it, then fix it with targeted edits, then update THREAT_MODEL.md with the new threat and control. Never fix by weakening a test.` })
  second = (await attack(2)).filter(Boolean).flatMap(r => r.attempts).filter(x => x.leaked)
  log(`Red team round 2: ${second.length} still leaking`)
}
return { results, first_round: first, leaks, fix, still_leaking: second }
