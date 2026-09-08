export const meta = {
  name: 'lazyslice-hardening',
  description: 'Phase 5: torture schemas, failure UX, performance, then a three-attacker red team with one fix round and re-attack',
  phases: [
    { title: 'Features', detail: 'section 14 phase-5 items: TUI screens, provisioning and rung 4, polymorphic inference, CI matrix and supply chain' },
    { title: 'Harden', detail: 'torture, failure UX, performance, each through implement.js in sequence' },
    { title: 'Red team', detail: 'three attackers try to make the tool leak; fix; re-attack' },
  ],
}
const REPO = '/Users/gareth/personal_repos/lazyslice'
const IMPL = `${REPO}/.claude/workflows/implement.js`
// args: { step: 'features' | 'harden' | 'redteam' }  one step per usage window
const step = (args && args.step) || 'features'

const FEATURES = [
  { id: 'T-TUI', title: 'Bubble Tea reasons and plan screens', stage: 'tui', paths: ['internal/tui/', 'cmd/lazyslice/'], integration: false,
    brief: `Implement internal/tui per docs/adr/002-tui.md and ARCHITECTURE.md §7 and §9: the reasons screen (classifier decisions with their reason strings, per-column opt-out that builds the same --unmask REASON flag) and the plan screen (tables, estimates, caps hit), both building a core.Request. The line printer stays the default; the TUI runs only on a TTY when the flag or the question ladder invokes it. Every action shows its keybinding; add the ADR-002 test that fails any keybinding lacking a CLI flag. Model tests with the framework's test utilities.` },
  { id: 'T-PROVISION', title: 'Provisioning --create-target and rung 4', stage: 'discover', paths: ['internal/discover/', 'cmd/lazyslice/'], integration: true,
    brief: `Implement discover/provision per ARCHITECTURE.md §9 "Provisioning": --create-target creates and starts a Postgres container matching the source major, and rung 4 offers stopped containers. The T2 gate runs on the provisioned target exactly as on any other. Tracker T-0063 records the seam T-DISCOVER left: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, restore the prompter and controlling-terminal read that were removed as unreachable, and wire --create-target and the one blocking question Q1 per ADR-008 section 6. Integration test against Docker; skip cleanly when Docker is absent.` },
  { id: 'T-POLY', title: 'Polymorphic association inference', stage: 'plan', paths: ['internal/plan/', 'internal/introspect/'], integration: false,
    brief: `Implement ARCHITECTURE.md §3.2 polymorphic inference: detect _type/_id and content_type_id/object_id pairs, add parent-direction virtual edges, report them in the plan, and keep the caps bounding them. Until now the plan printed "polymorphic pair detected, not followed: no constraint"; keep that message for pairs that inference cannot resolve. Tests on nasty.sql's polymorphic pair.` },
  { id: 'T-CI5', title: 'Five-major CI matrix, SBOM, govulncheck', stage: 'foundations', paths: ['.github/', 'Makefile', '.goreleaser.yaml'], integration: false,
    brief: `Per ADR-003 and THREAT_MODEL T10: the integration job runs against Postgres 14, 15, 16, 17, and 18 via testcontainers; add govulncheck to CI; add SBOM generation and checksum signing to the goreleaser release; add the docs-drift job (generated FLAGS.md, KEYBINDINGS.md, ERRORS.md must match tools/docgen output) and the unsafe-flag grep job from CONCEPT.md's enforcement list.` },
]

if (step === 'features') {
  phase('Features')
  const results = []
  for (const t of FEATURES) {
    const r = await workflow({ scriptPath: IMPL }, { ...t, model: t.id === 'T-CI5' ? 'sonnet' : 'opus', effort: 'high' })
    results.push(r)
    if (!r || r.status !== 'merged') { log(`Stopped at ${t.id}`); return { results, stopped_at: t.id } }
  }
  return { results }
}

const TASKS = [
  { id: 'T-TORTURE', title: 'Schema torture suite', stage: 'hardening', paths: ['testdata/torture/', 'internal/', 'Makefile', 'docs/TORTURE.md'], integration: true,
    brief: `Collect ten real open-source PostgreSQL schemas of different shapes: Discourse, GitLab (a trimmed subset is acceptable, say why), Mastodon, Odoo (subset), Metabase, Supabase auth schema, Cal.com, Plausible, a Django default project with auth and admin, and a Rails default with ActiveStorage. Put each under testdata/torture/<name>/ with its source URL and commit pinned in a README. Add a make torture target that loads each into a container with a small amount of generated data (a generator script per schema, a few hundred rows in the most-connected tables) and runs a snapshot from its most-connected table into a second container, then the invariant checks. For every failure, write a minimal reproduction into testdata/regressions/ before fixing it in internal/. Write docs/TORTURE.md with a table: schema, tables, run time, failures found and fixed, and for three schemas a hand-labelled PII truth set with precision and recall. Do not stop until at least nine of ten snapshot cleanly and the tenth fails with a message that names the exact cause.` },
  { id: 'T-FAILUX', title: 'Failure UX and error catalogue', stage: 'hardening', paths: ['internal/', 'cmd/', 'docs/ERRORS.md'], integration: true,
    brief: `Audit every error path. Each user-facing error must say what went wrong, which table or column, and the exact flag or action that fixes it. No stack trace reaches the user without --debug. Partial failures must leave the target either empty or complete; verify the drop-on-failure path with a test that kills the load mid-way. Define an error type per failure class in one package and write docs/ERRORS.md as the catalogue; add a test that every error type in the code appears in the catalogue and vice versa. Every fix is a targeted edit; do not restructure packages.` },
  { id: 'T-PERF', title: 'Performance baseline', stage: 'hardening', paths: ['internal/extract/', 'internal/load/', 'internal/transform/', 'docs/PERF.md', 'Makefile', '.github/'], integration: true,
    brief: `Profile a snapshot of 5,000 root rows from a source whose child table has 20,000,000 rows (extend the nasty.sql generator with a size parameter). Report where time goes per stage with pprof. Target: under 3 minutes on this machine against local containers. Optimise only the top two hotspots. Write the before and after numbers, the commands, and the flame summary into docs/PERF.md. Add a make bench target and a CI job that fails if extract throughput drops more than 20% from the recorded baseline stored in testdata/bench/baseline.json.` },
]

if (step === 'harden') {
  phase('Harden')
  const results = []
  for (const t of TASKS) {
    const r = await workflow({ scriptPath: IMPL }, { ...t, model: 'opus', effort: 'high' })
    results.push(r)
    if (!r || r.status !== 'merged') { log(`Stopped at ${t.id}`); return { results, stopped_at: t.id } }
  }
  return { results }
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
return { first_round: first, leaks, fix, still_leaking: second }
