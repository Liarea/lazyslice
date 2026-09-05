export const meta = {
  name: 'lazysnap-research',
  description: 'Phase 0 and 1: research documents, prompt guide digests, name check, adversarial critique, synthesis',
  phases: [
    { title: 'Research', detail: 'one agent per document, web research with sources' },
    { title: 'Critique', detail: 'adversarial reviewer per document' },
    { title: 'Revise', detail: 'author addresses findings' },
    { title: 'Guides', detail: 'digest one prompting guide per model' },
    { title: 'Synthesis', detail: 'ten facts, risks, contradictions with CONCEPT.md' },
  ],
}

const REPO = '/Users/gareth/personal_repos/lazysnap'

const PRE = `You are on the lazysnap team. Repo: ${REPO}. Read ${REPO}/CONCEPT.md and ${REPO}/CLAUDE.md first.
Rules: write ONLY the file(s) your task names, using absolute paths under ${REPO}. Do not edit any other file. Do not run git commit. Reddit cannot be fetched from this environment; use Hacker News (hn.algolia.com search works), GitHub issues and discussions, blogs, official docs, Stack Overflow, and dev.to instead. Every factual claim gets a markdown link to its source. If you cannot find a source, say "unverified" rather than inventing one. Recognizing a tool's name is not the same as knowing its current state: search for each tool as named and verify its status as of September 2026. Batch independent web fetches in one response. Do not stop until the file is written and complete; nobody is watching and nobody can answer questions. Your final message is not for a human: return only the structured output.`

const DOC = {
  type: 'object',
  required: ['file', 'summary', 'key_findings', 'sources_count', 'gaps', 'postmortem'],
  properties: {
    file: { type: 'string' },
    summary: { type: 'string', description: 'At most 120 words on what the document establishes' },
    key_findings: { type: 'array', items: { type: 'string' }, maxItems: 8 },
    sources_count: { type: 'integer' },
    gaps: { type: 'string', description: 'What could not be found or verified' },
    postmortem: { type: 'string', description: 'One line each: went well | went badly | change next time' },
  },
}

const CRIT = {
  type: 'object',
  required: ['verdict', 'issues', 'missing'],
  properties: {
    verdict: { type: 'string', enum: ['accept', 'revise'] },
    issues: { type: 'array', items: { type: 'object', required: ['severity', 'issue', 'fix'], properties: {
      severity: { type: 'string', enum: ['high', 'medium', 'low'] }, issue: { type: 'string' }, fix: { type: 'string' } } } },
    missing: { type: 'array', items: { type: 'string' }, description: 'Questions the document should answer but does not' },
  },
}

const DOCS = [
  { key: 'competitors', model: 'opus', file: 'research/COMPETITORS.md', prompt: `Write ${REPO}/research/COMPETITORS.md, a teardown of every tool in this space. Cover at minimum: Greenmask, Basecut, Neosync (verify its status after the August 2025 Grow Therapy acquisition), Snaplet snapshot (open-sourced 2024; verify activity), PostgreSQL Anonymizer, Jailer, condenser, replibyte, pgsubset, pg_sample, Tonic Structural, Redgate Data Masker, and any tool you discover that launched in 2025 or 2026. For each: language, supported databases, how subsetting works (algorithm, FK traversal direction, cycle handling), how masking is configured, install path, licence, last commit date, star count, and the three most-upvoted open issues with links. Then a table estimating "time from install to first usable snapshot" from reading each quickstart, with the number of config lines required. End with a section "What nobody does well" that cites evidence from the issues, and a section "What we must match to be credible".` },
  { key: 'postmortems', model: 'opus', file: 'research/POSTMORTEMS.md', prompt: `Write ${REPO}/research/POSTMORTEMS.md. Snaplet shut down in August 2024 and Neosync was acquired by Grow Therapy in August 2025. Read their archived docs, GitHub issues and discussions, changelogs, Hacker News threads (search hn.algolia.com), founder posts, pricing pages via the Wayback Machine, and interviews. Answer: what did users love, what did users abandon them over, what did the product become that it did not start as, where did complexity accumulate, what pricing did they try and how did it change, and what did the founders say about why. Quote users directly with links. Finish with "Five things we will not copy" and "Three things we must copy", each tied to evidence.` },
  { key: 'hardproblems', model: 'fable', file: 'research/HARD_PROBLEMS.md', prompt: `Write ${REPO}/research/HARD_PROBLEMS.md, a technical survey of the four hard problems in database subsetting and masking, aimed at the engineer who will implement them in Go next month. (1) Subsetting: walking FK graphs with cycles, self-references, composite keys, polymorphic associations without constraints, and "children of root" versus "everything reachable"; cite how Jailer, Greenmask, condenser and Snaplet approached it and any academic work on referential subsetting. (2) Deterministic masking: keeping the same fake value for the same input across tables via keyed hashing so joins on masked columns still work; format-preserving encryption options; unique-constraint collisions. (3) PII classification: column-name heuristics, value regexes and validators, entropy, multilingual names, JSON and free-text fields; where ML is and is not worth it; the categories a first release must catch. (4) Streaming extract and load at scale in Postgres: COPY protocol, server-side cursors, batching, insert ordering to satisfy constraints, deferrable constraints, sequences and identity columns, partitioned tables. For each problem: the simplest approach that is correct, then the refinement, then the trap that catches implementers. Link every source.` },
  { key: 'complaints', model: 'opus', file: 'research/COMPLAINTS.md', prompt: `Write ${REPO}/research/COMPLAINTS.md by mining real user complaints about getting realistic data into local and CI databases. Sources: hn.algolia.com search, GitHub issues on Greenmask, Neosync, Snaplet, pg_anonymizer, Jailer and replibyte, Stack Overflow questions tagged database-testing, anonymization, or test-data, dev.to, and engineering blogs. Collect at least 25 verbatim quotes, each with a link and a date, tagged by theme: config burden, speed, broken foreign keys, leaked PII, unsupported database, CI integration, trust. Rank themes by frequency in a table. Add a short section on which themes the lazysnap principles in CONCEPT.md address and which they do not.` },
  { key: 'licence', model: 'opus', file: 'research/LICENSE_DECISION.md', prompt: `Write ${REPO}/research/LICENSE_DECISION.md comparing Apache-2.0, MIT, AGPL-3.0, and BSL 1.1 for an open-core CLI whose paid product is a hosted service. Cover: contributor comfort, corporate adoption friction, protection against a cloud vendor hosting the tool, what Greenmask, Neosync, Snaplet, lazygit and sqlit chose and why (verify each), how each licence interacts with a future closed hosted layer in the same repo versus a separate repo, and CLA or DCO. Recommend one licence and one repo structure with reasoning. One page.` },
  { key: 'sqlit', model: 'opus', file: 'research/SQLIT_STUDY.md', prompt: `Clone https://github.com/Maxteabag/sqlit into /private/tmp/claude-501/lazysnap-scratch/sqlit (create the directory) and read it, along with lazygit's README and keybinding docs. Write ${REPO}/research/SQLIT_STUDY.md describing exactly how sqlit achieves a zero-config first run: Docker container detection (quote the code path), connection saving, keyring use, keybinding discoverability, autocomplete, and how it presents errors. Note what in its repo structure, README, release process, and contributor docs made it easy to adopt. Then specify lazysnap's equivalent, step by step: what happens when a user types "lazysnap" with no arguments in a folder that has a docker-compose.yml, a DATABASE_URL in .env, or nothing at all. Define every question the tool may ask and its default, capped at one question on the happy path.` },
  { key: 'practices', model: 'opus', file: 'research/AI_PROJECT_PRACTICES.md', prompt: `Write ${REPO}/research/AI_PROJECT_PRACTICES.md. Study open-source projects that grew fast in 2025 and 2026 while being built largely by one person with AI coding agents (sqlit is one; find at least six others, and verify each claim of AI-assisted authorship from the author's own words). For each: what their CLAUDE.md, AGENTS.md, or contributor docs contain, how they gate contributions, how they structure tests so agents cannot fake success, their release automation, and what their launch looked like (Show HN, Reddit, GIF in README). Then study how mature multi-agent coding setups keep quality: guardrail files per directory, invariant tests, review bots, and "definition of done" conventions. Finish with a concrete list of practices to adopt for lazysnap, each with the source that showed it working, and a list of practices to avoid, with the failure it caused somewhere.` },
  { key: 'name', model: 'opus', file: 'NAME.md', prompt: `The working name is lazysnap. Check it across GitHub (repos and orgs), PyPI, npm, crates.io, Homebrew formulae, Go module proxies, the USPTO TESS search, and .dev/.sh/.io domain availability (use a WHOIS or RDAP lookup via curl, for example https://rdap.org/domain/lazysnap.dev). Report every collision with a link. Then propose five alternative names that keep the "lazy" family association, are pronounceable, are short to type, and have at least one free domain among .dev, .sh, .io. Write ${REPO}/NAME.md with a recommendation and its reasoning. Do not buy anything.` },
]

const GUIDES = [
  { key: 'fable-5-1', url: 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1', model: 'Claude Fable 5.1' },
  { key: 'opus-5', url: 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5', model: 'Claude Opus 5' },
  { key: 'sonnet-5', url: 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5', model: 'Claude Sonnet 5' },
  { key: 'opus-4-8', url: 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-4-8', model: 'Claude Opus 4.8' },
  { key: 'general', url: 'https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices', model: 'all current models, plus whatever the page says about Haiku 4.5' },
]

const critic = (d) => (r) => {
  if (!r) return null
  return agent(`${PRE}
You are an adversarial reviewer. Read ${REPO}/${d.file} in full. Your job is to find what is wrong or missing, not to praise. Check: every factual claim has a working link; tool statuses are current as of September 2026, not stale; no competitor is described from memory; no section is padding; the document answers every question its brief asked (the brief: """${d.prompt}"""). Spot-check at least five links by fetching them. Return verdict "revise" if any high-severity issue or any unanswered brief question exists.`,
    { label: `critique:${d.key}`, phase: 'Critique', model: 'opus', schema: CRIT }).then(c => ({ author: r, critique: c }))
}

const revise = (d) => (x) => {
  if (!x || !x.critique) return x ? x.author : null
  if (x.critique.verdict === 'accept' && !x.critique.issues.some(i => i.severity === 'high')) return x.author
  return agent(`${PRE}
You wrote ${REPO}/${d.file}. A reviewer found these issues: ${JSON.stringify(x.critique.issues)}. Missing: ${JSON.stringify(x.critique.missing)}. Fix every high and medium issue and answer every missing question, editing the file in place with targeted edits. Do not remove sourced content to make the file shorter. Return the updated structured output.`,
    { label: `revise:${d.key}`, phase: 'Revise', model: 'sonnet', effort: 'medium', schema: DOC })
}

log('Research: 8 documents, each critiqued and revised; 5 prompt guides digested in parallel')

const [docs, guides] = await parallel([
  () => pipeline(DOCS,
    d => agent(`${PRE}\n${d.prompt}`, { label: `research:${d.key}`, phase: 'Research', model: d.model, schema: DOC }),
    (r, d) => critic(d)(r),
    (x, d) => revise(d)(x),
  ),
  () => pipeline(GUIDES,
    g => agent(`${PRE}
Fetch ${g.url} (WebFetch; if the page is very long, fetch it in more than one pass with different extraction prompts). Write ${REPO}/docs/prompting/${g.key}.md: an operational cheat sheet for someone writing prompts to ${g.model} inside an automated coding workflow. Sections: "What is different about this model" (from the page, not from memory); "Prompt skeleton" (a template with role, context, task, constraints, definition of done, return format); "Phrases that work" (verbatim from the page where it gives them); "Anti-patterns"; "Effort and length guidance"; "For coding agents" (finishing tasks, scope, tests, edits, subagents, batching, whatever the page says). Every recommendation must trace to the page. Under 900 words.`,
      { label: `guide:${g.key}`, phase: 'Guides', model: 'opus', effort: 'medium', schema: DOC }),
  ),
])

const good = docs.filter(Boolean)
log(`Research done: ${good.length}/${DOCS.length} documents, ${guides.filter(Boolean).length}/${GUIDES.length} guides`)

const index = await agent(`${PRE}
Read every file in ${REPO}/docs/prompting/. Write ${REPO}/docs/prompting/README.md: a one-screen index, then a "Model routing" table copied from ${REPO}/docs/OPERATING_MODEL.md, then five ready-to-use prompt templates for this project's agent roles (researcher, developer, reviewer, mechanic, synthesiser), each written for the model that role uses per the routing table and following that model's cheat sheet. Templates use {placeholders} for repo path, files, task, definition of done. Under 1200 words.`,
  { label: 'guides:index', phase: 'Guides', model: 'opus', effort: 'medium', schema: DOC })

const synth = await agent(`${PRE}
Read every file in ${REPO}/research/ and ${REPO}/NAME.md. Write ${REPO}/research/SYNTHESIS.md: (1) the ten facts that most change what we build, each with a link back to the research document and line or heading it came from; (2) three risks that could kill the project, each with a mitigation; (3) a revised non-goals list; (4) every statement in ${REPO}/CONCEPT.md that the research contradicts or weakens, quoted, with what the research says instead; (5) recommendations that phase 2 architecture must take a position on, as a numbered list of questions. Do not soften findings. Then apply the CONCEPT.md corrections from section 4 directly to ${REPO}/CONCEPT.md with targeted edits, keeping it under 700 words.`,
  { label: 'synthesis', phase: 'Synthesis', model: 'fable', effort: 'high', schema: {
    type: 'object', required: ['ten_facts', 'risks', 'concept_edits', 'questions_for_architecture', 'postmortem'],
    properties: { ten_facts: { type: 'array', items: { type: 'string' } }, risks: { type: 'array', items: { type: 'string' } },
      concept_edits: { type: 'array', items: { type: 'string' } }, questions_for_architecture: { type: 'array', items: { type: 'string' } },
      postmortem: { type: 'string' } } } })

const complete = await agent(`${PRE}
You are the completeness critic. Read ${REPO}/research/SYNTHESIS.md and skim every file in ${REPO}/research/. Answer: what question about building lazysnap would a cautious engineering lead ask that these documents do not answer? Which research document is thinnest on evidence? Which "fact" in SYNTHESIS.md rests on a single unverified source? List at most eight items, each with the file that should answer it.`,
  { label: 'completeness', phase: 'Synthesis', model: 'opus', schema: { type: 'object', required: ['items'], properties: { items: { type: 'array', items: { type: 'string' } } } } })

return { docs: good, guides: guides.filter(Boolean), index, synth, completeness: complete ? complete.items : [] }
