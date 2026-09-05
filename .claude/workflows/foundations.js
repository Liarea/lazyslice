export const meta = {
  name: 'lazysnap-foundations',
  description: 'Phase 3: scaffold, then fixtures, invariants, per-directory CLAUDE.md and roadmap in parallel, then review',
  phases: [
    { title: 'Scaffold', detail: 'Go module, interfaces from ARCHITECTURE.md, Makefile, CI, release' },
    { title: 'Parallel', detail: 'fixtures, invariant tests, CLAUDE.md files, roadmap' },
    { title: 'Review', detail: 'can the invariants be faked, are all traps present, does CI run integration' },
  ],
}
const REPO = '/Users/gareth/personal_repos/lazysnap'
const IMPL = `${REPO}/.claude/workflows/implement.js`

phase('Scaffold')
const scaffold = await workflow({ scriptPath: IMPL }, { id: 'T-SCAFFOLD', title: 'Repository scaffold', model: 'opus', stage: 'foundations', paths: ['everything except research/, docs/adr/, tracker/, CONCEPT.md'],
  brief: `Scaffold the Go repository exactly per ${REPO}/ARCHITECTURE.md (repository layout, dependency list, interfaces). Concretely: go mod init github.com/Liarea/lazysnap (Go version from your toolchain); cmd/lazysnap/main.go with the CLI framework named in ARCHITECTURE.md and the v1 flag surface registered but every stage a documented no-op; one package per pipeline stage under internal/ with the interfaces from ARCHITECTURE.md as real Go code, a doc comment, and a no-op implementation; internal/testutil with a helper that starts a PostgreSQL 16 container via testcontainers-go and returns a connection URL; add every dependency ARCHITECTURE.md lists with go get at the stated versions so later agents never touch go.mod; a Makefile with build, test, lint, integration (integration runs go test -tags integration ./...), fmt, and check (= lint test) targets; .golangci.yml with a sane strict set; .github/workflows/ci.yml running lint and test on push and integration on a docker-enabled runner; .github/workflows/release.yml on tags with goreleaser; .goreleaser.yaml producing darwin/linux/windows amd64 and arm64 binaries, checksums, and a Homebrew tap stanza pointing at github.com/Liarea/homebrew-tap; SECURITY.md with a private PII-miss reporting route; CONTRIBUTING.md summarising the rules in CLAUDE.md; a minimal README.md placeholder that says the project is pre-release. Everything must pass make lint and make test on the empty implementation. Then, and only then, run: git add -A && git commit -m "foundations: scaffold (T-SCAFFOLD)" (this task is the exception to the no-commit rule so parallel work can branch from it).` })
if (!scaffold || scaffold.status !== 'merged') return { scaffold, stopped: 'scaffold did not merge' }

phase('Parallel')
const tasks = [
  { id: 'T-FIXTURES', title: 'Golden fixtures: Pagila and nasty.sql', model: 'opus', stage: 'foundations', paths: ['testdata/', 'internal/testutil/fixtures.go'],
    brief: `Build ${REPO}/testdata/ with two PostgreSQL fixtures. (1) Pagila: download the schema and data SQL from github.com/devrimgunduz/pagila (pin the commit in testdata/README.md) into testdata/pagila/. (2) testdata/nasty.sql, a hostile schema you design, containing: a self-referencing table; a three-table foreign-key cycle; a composite primary key with a composite foreign key to it; a polymorphic association (owner_type, owner_id) with no constraint; a JSONB column containing emails and phone numbers nested two levels deep; a free-text notes column containing full names; a boolean column named email_verified (false-positive trap); a column named ref holding email addresses (false-negative trap); a partitioned table with two partitions; an enum type; a text[] array column with emails; a generated column; identity columns with non-default sequence starts; a table in a schema other than public; a quoted mixed-case identifier; and a function that generates 2,000,000 rows into a table for streaming tests, gated behind a psql variable so normal loads stay fast. Add internal/testutil/fixtures.go with LoadPagila(ctx, url) and LoadNasty(ctx, url, big bool) using the existing testutil container helper. Document every trap in testdata/README.md with the behaviour lazysnap must show for it. Add a unit test that loads both fixtures into a container and asserts table counts.` },
  { id: 'T-INVARIANTS', title: 'Invariant test suite I1 to I6', model: 'opus', stage: 'foundations', paths: ['internal/invariants/'],
    brief: `Write ${REPO}/internal/invariants/, a black-box integration suite (build tag integration) that runs the built lazysnap binary against two PostgreSQL containers from internal/testutil and both fixtures, and asserts the six invariants in ${REPO}/ARCHITECTURE.md: I1 every foreign key in the target resolves (query information_schema and check each FK with an anti-join); I2 no flagged column in the target contains a value the classifier's own detectors would flag (call the classify command with --json on the target and assert zero hits, and additionally grep for every email and phone that existed in the source); I3 same source, same secret, same config yields a byte-identical target (pg_dump --data-only both targets and compare); I4 the source is unchanged (row counts and a per-table md5 of ordered rows before and after); I5 the emitted lazysnap.yml, fed back in, reproduces the snapshot (compare with I3's method); I6 the root table row count equals --take. Structure it so each invariant is one test function that fails with a message naming the table and column. Since the pipeline is a no-op today, every test must currently FAIL for the right reason, not skip; add a helper that skips only when Docker is unavailable. Wire make integration to run this package.` },
  { id: 'T-CLAUDEMD', title: 'Per-directory CLAUDE.md files', model: 'sonnet', stage: 'foundations', paths: ['cmd/CLAUDE.md', 'internal/CLAUDE.md', 'internal/*/CLAUDE.md', 'testdata/CLAUDE.md', 'docs/CLAUDE.md', 'tracker/CLAUDE.md', '.claude/CLAUDE.md'],
    brief: `Write a CLAUDE.md in each of: cmd/, internal/, each internal/<stage>/ package, testdata/, docs/, tracker/, and .claude/. Each is under 40 lines and states: what lives here and what does not; the interface or contract the directory implements, pointing at the exact type in ${REPO}/ARCHITECTURE.md; the rules specific to it (for example internal/transform: every masker must be deterministic under the HMAC scheme and must be registered in the masker table; internal/load: never connect to the source; testdata: every trap has a README entry); how to test just this directory; and what a change here must never do. Read ARCHITECTURE.md, THREAT_MODEL.md, and the root CLAUDE.md first and do not contradict them.` },
  { id: 'T-ROADMAP', title: 'ROADMAP.md with gates and Later', model: 'sonnet', stage: 'foundations', paths: ['ROADMAP.md'],
    brief: `Write ${REPO}/ROADMAP.md from phases 4 to 8 of ${REPO}/docs/BUILD_PLAN.md. For each phase: the goal in one sentence, the gate as a checkbox list, and a "Not in this phase" list. Top section "Current phase: 3, Foundations" with the phase 3 gate. Add the rule at the top: any feature request goes into the Later section at the bottom with a one-line reason, and nothing moves out of Later until the current gate is ticked. Seed Later with the v1 non-goals from CONCEPT.md.` },
]
const results = await parallel(tasks.map(t => () => workflow({ scriptPath: IMPL }, t)))

phase('Review')
const REV = { type: 'object', required: ['findings'], properties: { findings: { type: 'array', items: { type: 'object', required: ['severity', 'file', 'issue', 'fix'], properties: { severity: { type: 'string', enum: ['high', 'medium', 'low'] }, file: { type: 'string' }, issue: { type: 'string' }, fix: { type: 'string' } } } } } }
const reviews = await parallel([
  'Could a lazy or dishonest implementation of the pipeline make the invariant suite in internal/invariants pass without actually subsetting, masking, or verifying? For each invariant, describe the cheapest fake that passes and whether the test catches it.',
  'Compare testdata/README.md and testdata/nasty.sql against the trap list in the T-FIXTURES brief in this workflow script and the classifier design in ARCHITECTURE.md. Which traps are missing, mislabelled, or would not actually trigger the failure they claim?',
  'Read .github/workflows/*.yml, Makefile, and .goreleaser.yaml. Will CI actually run the integration suite with Docker available? Will a tag actually produce binaries and a Homebrew formula? Will a fresh clone pass make check? Run make check yourself.',
].map((q, i) => () => agent(`You are a reviewer on lazysnap. Repo: ${REPO}. Read ${REPO}/CLAUDE.md and ${REPO}/ARCHITECTURE.md. ${q} Return findings; high severity means the phase 3 gate is not met.`,
  { label: `review:${i + 1}`, phase: 'Review', model: 'opus', schema: REV })))
const findings = reviews.filter(Boolean).flatMap(r => r.findings).filter(f => f.severity !== 'low')

let fixes = null
if (findings.length) {
  log(`${findings.length} findings from foundation review; one fix task`)
  fixes = await workflow({ scriptPath: IMPL }, { id: 'T-FOUNDFIX', title: 'Foundation review fixes', model: 'opus', stage: 'foundations', paths: ['testdata/', 'internal/', '.github/', 'Makefile', '.goreleaser.yaml'],
    brief: `Address every finding below with targeted edits. Where a finding says a test could be faked, strengthen the test. Findings:\n${JSON.stringify(findings, null, 1)}` })
}
return { scaffold, results, findings, fixes }
