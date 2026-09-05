# ADR-000: The project is named lazyslice

Status: accepted, 2026-09-05

## Context

The working name was lazysnap. The name audit in NAME.md found: a Go TUI already published at github.com/jpdarago/lazysnap (v0.2.0, June 2026) in the same "lazy" family; the npm package and @lazysnap scope taken; the GitHub account LazySnap taken; lazysnap.com registered; and DBSnapper, a live commercial product, selling "snapshot, subset, sanitize" for Postgres, so "snap" in this category reads as their brand. Separately, a snapshot means a whole database at a point in time, while this tool's entire claim is a small referentially complete slice. The name argued with the pitch.

## Options considered

- Keep lazysnap and avoid the collisions by owner path. Does not fix the semantic problem or the DBSnapper adjacency.
- lazysubset. Clean on every registry and domain; the category's term of art; one character longer and drier.
- lazyslice. Names the mechanism, is the word Jailer and Tonic already use for the output, and CONCEPT.md's own transcript said "23 tables in slice" before anyone chose it. Collisions are a dormant Haskell org (2020) and a scientific Python package we will never ship against. lazyslice.dev, .sh, and .io are free.

## Decision

lazyslice. Canonical domain lazyslice.dev when the human buys one; nothing has been purchased. Repository path github.com/Liarea/lazyslice. Binary and Homebrew formula lazyslice. Config file lazyslice.yml. Secret file lazyslice.secret.

## Consequences

Every file, path, and workflow constant that said lazysnap changes in one commit. The local repository directory moves to ~/personal_repos/lazyslice. Research documents written before this ADR keep "lazysnap" in their prose where quoting it; that is history, not error.

## Reversal condition

If the human prefers lazysubset or another name before the first public release, rename again; the cost is one commit. After the first public release, the name is fixed.
