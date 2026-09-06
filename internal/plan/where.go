// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"strconv"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// --where is the one part of a planner statement a person outside this program
// wrote, and internal/pg's {where} placeholder bounds what it may contain: no
// `;`, no `--`, no `/*`, no `\`, no `$`, no `-` or `/` left dangling, and
// parentheses that balance within the predicate to whereMaxDepth levels
// (internal/pg/tracer.go).
//
// The tracer is the backstop and not the message. A predicate that breaks the
// rule never reaches the server, but what the operator sees is
// "plan: reading the root's keys from public.orders: pg: the source refused a
// statement: pg: statement does not match any registered shape on the source",
// which names neither --where nor the character that did it — and an ordinary
// Postgres predicate can break the rule by accident: `email ~ '^\w+@example\.com$'`
// carries both a backslash and a dollar sign. Documentation is not the fix for
// that (CLAUDE.md); refusing at the flag is.
//
// The second reason is the trace. A tracer refusal increments Violations() and
// fills Violation(), and that is recorded in the trace and printed in the
// report as a T9 allowlist violation. An operator's typo landing there is
// indistinguishable from a statement one of our own bugs generated. Checking
// the same rule here, before any statement is built, is what keeps a recorded
// violation meaning "lazyslice generated a statement it should not have".
//
// The rule reads the text and not the SQL: a `;`, a `(` or a `)` inside a
// string literal in the predicate is refused with the rest, because telling a
// literal from structure costs a SQL lexer and the refusal costs a rephrasing.
//
// This duplicates internal/pg's rule rather than importing it: ARCHITECTURE.md
// §2's import graph has the stage packages importing pipeline and nothing else
// of the tree. The two are held together by TestTheWhereCheckAndTheShapeAgree
// in shapes_test.go, which runs both over one list of predicates.

// whereMaxDepth is internal/pg's whereMaxDepth: how deeply a predicate's own
// parentheses may nest before the shape stops admitting it.
const whereMaxDepth = 6

// whereFlag is the flag this check refuses, as the refusal prints it.
const whereFlag = "--where"

// checkWhere refuses a --where predicate the source allowlist would refuse,
// naming the character and its position. It returns nil for an empty predicate:
// no --where is not a predicate, and seedSQL writes no WHERE clause for it.
//
// The position is a 1-based character offset, and the reason is the offending
// character or pair. Neither is a value from the source and neither carries the
// predicate text (THREAT_MODEL.md T4, T5: `where` literals are withheld) — the
// characters this refuses are punctuation that cannot be a row.
func checkWhere(where string) *Refusal {
	runes := []rune(where)
	depth := 0
	for i := 0; i < len(runes); i++ {
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch runes[i] {
		case ';':
			return refuseWhere(i, "a statement separator `;`")
		case '\\':
			return refuseWhere(i, "a backslash")
		case '$':
			return refuseWhere(i, "a dollar sign")
		case '(':
			depth++
			if depth > whereMaxDepth {
				return refuseWhere(i, fmt.Sprintf("parentheses nested deeper than %d", whereMaxDepth))
			}
		case ')':
			depth--
			if depth < 0 {
				return refuseWhere(i, "a `)` that closes no `(` of its own")
			}
		case '-':
			if next == '-' {
				return refuseWhere(i, "a line comment `--`")
			}
			if !whereFollowsDash(next) {
				return refuseWhere(i, "a `-` with nothing a predicate may follow it with")
			}
		case '/':
			if next == '*' {
				return refuseWhere(i, "a block comment `/*`")
			}
			if !whereFollowsSlash(next) {
				return refuseWhere(i, "a `/` with nothing a predicate may follow it with")
			}
		}
	}
	if depth > 0 {
		return refuseWhere(len(runes), "a `(` it never closes")
	}
	return nil
}

// whereOrdinary is internal/pg's reWhereOrd: a character a predicate may carry
// on its own.
func whereOrdinary(r rune) bool {
	switch r {
	case 0, ';', '\\', '$', '/', '(', ')', '-':
		return false
	}
	return true
}

// whereFollowsDash and whereFollowsSlash are the "admitted only together with
// the character after it" halves of reWhereDash and reWhereSlash. A `-` or `/`
// at the very end of the predicate has no following character and is refused
// with them, which is a syntax error on the server either way.
func whereFollowsDash(next rune) bool  { return whereOrdinary(next) || next == '(' }
func whereFollowsSlash(next rune) bool { return (whereOrdinary(next) && next != '*') || next == '(' }

// refuseWhere builds the exit-2 refusal. The table is the zero TableRef: this
// is a fault in the request, not in any table of the source.
func refuseWhere(at int, reason string) *Refusal {
	return refuse(CodeWhereSyntax, exitUsage, ref.TableRef{},
		fmt.Sprintf("%s carries %s at character %d", whereFlag, reason, at+1),
		event.Args{
			event.ArgFlag:   whereFlag,
			event.ArgReason: reason,
			event.ArgCount:  strconv.Itoa(at + 1),
		})
}
