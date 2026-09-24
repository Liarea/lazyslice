# ADR-016: A committed target record of lazyslice's own container names the container, not its port

Status: proposed, 2026-09-24. Amends ADR-008 §1 (what rung 0 short-circuits with) and §6 (the reach of `--create-target`) for exactly one kind of committed record: a `target:` block that names a container lazyslice created. ADR-008 is accepted and frozen and stands for every other record, flag and question; this is a new record, not an edit (root CLAUDE.md).

## Context

Dogfood session 2 (tracker T-0327, GitHub issue #173) copied session 1's `lazyslice.yml` to a new directory and ran with only `--source`. The file's `target:` block read `from: container`, `service: lazyslice-target-<project>`, with the container's host, port, role and database. The run stopped at exit 4 with the driver's raw `password authentication failed for user "postgres" (SQLSTATE 28P01)`, rendered under a row that says the target "did not respond".

The cause is what ADR-008 §1 makes a committed record mean: an address. Rung 0 rebuilds a DSN from the reference and leaves the password to `PGPASSWORD`, `~/.pgpass` and `--password-command` (ADR-008 §6, Q4). That covers a database the developer administers. It does not cover the one lazyslice creates itself: `--create-target` mints a random `POSTGRES_PASSWORD` whose homes are the container's own environment and the machine-local state dir (ARCHITECTURE.md §9 "Provisioning", ADR-004). `internal/discover`'s `rung0Target` reads the state dir, keyed by the container name *this directory's* project would produce, and compares it with the file's label rather than taking the label as a path (internal/discover/CLAUDE.md, "A provisioned target's credential is recovered at rung 0"). In a new directory the two names differ, so nothing was read. On a second machine the container and the state dir do not exist at all.

Two further observations from the same session:

- `--create-target` did not help. ADR-008 §6 scopes it to Q1's headless answer, and §1 does not list it among what outranks rung 0, so the run failed the same way (internal/discover/CLAUDE.md, "`--create-target` answers Q1 and does nothing else", which names a superseding ADR as the only way to widen it).
- When the maintainer passed `--target` with the password, the gate refused. That refusal was correct: the target held lazyslice's copy of a *different* source, so its marker was not bound to this one (ARCHITECTURE.md §11.2, ADR-005 rule 4) and rule 5 found it not empty. But the line rendered a literal `{table}` placeholder and gave the table list as its reason. It never said that the rows were lazyslice's own copy of another source.

## Options considered

**(A) Keep the record an address. Document `docker exec … printenv POSTGRES_PASSWORD` and `--password-command`.** This costs nothing in code. It is documentation as the fix for a confusing run, which root CLAUDE.md forbids, and it leaves the second-machine case with nothing to connect to at all.

**(B) Record the password, or a pointer to it, in `lazyslice.yml`.** Rejected outright: the file is committed, and ADR-004 and THREAT_MODEL.md A3/T5 keep every credential out of it.

**(C) Key the state dir by the recorded label instead of this directory's project.** This fixes the copied-directory case on one machine and nothing else. It also reverses the rule that committed text never becomes a path this process reads. `provision.Password`'s single-path-element check would still stop a traversal, but it would then be the only thing standing between the file and the read.

**(D) Treat a record of lazyslice's own container as naming that container.** Look it up by name on a local Docker endpoint and read its credential from its own environment, the way rung 3 already reads any Postgres container's (THREAT_MODEL.md A3 names container env as a place a credential lives). When the container is absent, offer or refuse the one thing that fixes it. Chosen.

**(E) As (D), and make `--create-target` outrank every committed record, or the ladder's tie-break too.** Rejected. A record of a compose service or an environment variable names a database the developer owns, and letting a creation flag discard it is the widening internal/discover/CLAUDE.md already reverted once. The tie-break is ADR-008 §5's and is not what this session hit.

## Decision

(D).

1. **Which records.** A committed target record is *lazyslice's own container* when `from: container` is set, `service:` has the shape `provision.Name` produces (`lazyslice-target-` followed by characters of Docker's container-name alphabet), and the recorded host is loopback. Every other record keeps ADR-008 §1's meaning unchanged. The shape proves nothing about ownership on its own, because the file is committed text anybody can write. So the container that answers to the name must also carry the `lazyslice.project` label that `internal/discover/provision` stamps on everything it creates before its environment is read. A container with that name and without that label is somebody else's, and the record falls back to being an address.

2. **When the flag and the file disagree.** `--create-target` outranks such a record. The flag and the record name the same kind of thing, lazyslice's container, and the flag names this directory's: the run provisions `lazyslice-target-<this project>`, or reuses it when it already exists (ARCHITECTURE.md §9 "Provisioning"), with no question. It is refused before any container work on an endpoint that is not local or does not answer, exactly as ADR-008 §3 refuses it elsewhere. `--target`, the positional DSN and `--reconfigure` are unchanged and still outrank the file.

3. **Otherwise, on a local and reachable Docker endpoint** (ADR-008 §3's predicate, unchanged), the container is looked up by the recorded name, read-only, inside the 2 s listing budget:
   - **Running and ours:** the target is the container's live published binding, with the role and password from its own environment and the database the file records. The port is the container's rather than the file's on purpose: the record names the container, and the binding is where that container answers today. `lazyslice.yml` is rewritten with the new port at the end of a successful run, as it always is. The target counts as named by the operator, so ADR-013's same-cluster escalations do not apply to it, the same as for any committed target.
   - **Stopped and ours:** ADR-008 §6's Q1′, unchanged in wording, default and headless answer. Once started, the target is the database the file records, as it is for a running container, so whether the container was running does not decide which database is written.
   - **Absent:** ADR-008 §6's Q1, asked with a first sentence that names the missing container (`./lazyslice.yml's target is the container <name>, which this machine's docker does not have. start one? postgres:<major> as lazyslice-target-<project> on port <free> [Y/n]`). Yes creates this directory's container. A headless run, or a "no", stops at **exit 4** with the new event code `target.refused.container_missing`, naming the container, `--create-target` and `--target`. This is not a new question: it is Q1 in one more no-target state, so the one-question rule and ADR-008 §7's controlling-terminal rules apply to it unchanged.

4. **No usable Docker endpoint** (not local, not reachable, or a listing that fails): the record is an address again, as ADR-008 §1 had it. The state-dir lookup `rung0Target` already does still runs. Nothing is created, and nothing is refused on grounds Docker could not confirm.

5. **A refused password is never a bare SQLSTATE.** When the target answers and rejects the password (SQLSTATE 28P01), the run stops at exit 4 under the new code `target.refused.auth`. Its message names the endpoint, the role and every place a password comes from (the `--target` string, `$PGPASSWORD`, `~/.pgpass`, `--password-command`). Before this it was `target.refused.unreachable`, whose row says "did not respond". 28000 keeps that row and the server's own line, because a missing `pg_hba.conf` entry is not a password problem.

6. **The not-empty refusal says why.** `target.refused.not_empty`'s row becomes `the target {database} is not empty ({table}): {reason}`, and every argument is filled. When lazyslice's own marker is present and not bound to this source, `{reason}` says the target holds *a copy of another source*, or tables changed after lazyslice loaded them. `internal/pg` reports that a marker is unbound but not why, so the sentence covers both causes rather than guessing. Both variants name `--target`. The gate's rules, order and verdicts are unchanged (ADR-005, ADR-008 §5).

Nothing here relaxes the gate. A re-discovered, restarted or freshly provisioned container still passes all five of ADR-005's rules or is refused. That refusal is exactly what stops session 2's copied file from loading into session 1's copy of a different source.

## Consequences

- `internal/discover` does one Docker listing and one inspect for a committed record of its own container, including on a run that named its source, where ADR-008 §1 made no Docker call at all. No other record and no other run gains a call.
- `provision.IsName` is exported so that the shape test in §1 has one spelling, beside `provision.Name`.
- `Result.TargetContainerID` is set for a re-discovered container, so the post-run `target.connect.container` line names the real container rather than falling back to the generic line.
- `internal/event/catalogue.yml` gains `target.refused.container_missing` and `target.refused.auth`, both inside exit 4. `target.refused.not_empty`'s template changes. `docs/ERRORS.md` is regenerated from the catalogue.
- internal/discover/CLAUDE.md's notes "`--create-target` answers Q1 and does nothing else" and "A provisioned target's credential is recovered at rung 0" are amended to point here. The state dir is now the fallback for a lazyslice container record, not its first answer.
- ARCHITECTURE.md §9 gains an amendment paragraph after "Provisioning" pointing here.
- Tests: `TestACommittedContainerTargetIsTheContainerNotThePort` (unit: present, not ours, missing and headless, missing with `--create-target`, missing with a terminal), `TestASecondRunOpensTheTargetItProvisioned` (real daemon: the same record read from the directory that made it and from a different one), `TestATargetHoldingAnotherSourcesCopyIsRefusedAsOne` (real database: a target marked by one source, refused for a second with the words "a copy of another source" and nothing written), and `TestARefusedPasswordSaysWhereAPasswordComesFrom` / `TestTheNotEmptyRefusalRendersWhole` (the two rows render with no placeholder).
- Owed, outside this ADR's implementing paths (T-0370): `internal/pg` should report *why* a marker is unbound (another source, a newer schema version, or a changed catalog), so the not-empty refusal can say which one instead of both.

## Reversal condition

- If a dogfood session or issue report shows a run that reconnected to a re-discovered container the operator did not mean. For example, a container of the recorded name, carrying lazyslice's label, belongs to a different checkout on the same machine, and the gate passed it because it was empty. In that case the record goes back to being an address that also requires the recorded port to match, and this ADR is superseded.
- If two dogfood sessions end with a `lazyslice-target-*` container created through this ADR's Q1 that the human did not want, the missing-container state stops asking and only refuses. That is ADR-008's own Q1 reversal condition, applied to the one state this ADR adds.
- If `internal/pg` gains the unbound-marker reason (Consequences, last item), §6's two-cause sentence is replaced by the one cause, without a new ADR: it is wording, and the verdict does not change.
