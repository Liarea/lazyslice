// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"errors"
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The event code the transform stage renders. Transformer.Transform takes no
// event.Sink — core.Run is the only producer of events (ARCHITECTURE.md §7) —
// so this is declared here, beside the refusal that carries it, and emitted by
// core from the *Refusal Transform returns. It has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
//
// The refusal names a column and the masker id, both identifiers. It never
// carries the value that could not be masked: an error string is a path out of
// the process and THREAT_MODEL.md T4 closes it.
const (
	// CodeMasker is a masker refusing a value it was handed. It is a hard stop
	// and never a pass-through: a cell the masker cannot mask is the cleartext
	// T12 exists to keep out of the target, and copying it through under a
	// column the report calls masked is the failure mode that row describes.
	CodeMasker event.Code = "transform.refused.masker"
)

// exitTransform is the exit code ADR-005's table gives the stages that move
// rows ("7 extract or load"); transform sits between the two and a refusal here
// stops the same movement. ADR-005 does not name transform separately, and
// internal/transform/CLAUDE.md records the reading.
const exitTransform = 7

// Refusal is a transform failure core can render from the catalogue.
type Refusal struct {
	Code   event.Code
	Exit   int
	Col    ref.ColumnRef
	Masker string
	// Path is the JSON path of the leaf that failed, "" for a whole column.
	Path string
	// Reason is the wrapped error. It is not rendered into the event — the
	// catalogue's template names the column and the masker only — and Error()
	// renders it only when the mask module wrote it: a masker's own message is
	// written by the masker, an array-literal parse failure quotes the
	// literal, and either is a free-form string with a row value in it
	// (T-0191).
	Reason error
}

// Error names the column, the masker and either the mask module's own refusal
// (value-free by construction) or the *type* of any other reason, never a
// free-form reason's own words.
//
// The doc comment on this file has always promised "it never carries the value
// that could not be masked", and until T-0191 only the event kept that promise:
// Error() interpolated the reason with %v and cmd/lazyslice printed it at any
// verbosity. That is the same egress the 2026-09-15 panic amendment closed
// (THREAT_MODEL.md T4), through the one door the amendment did not check — a
// masker's message was withheld when it arrived as a panic and printed when it
// arrived as an error, and the two are the same string.
//
// The reason's own words are still reachable, by ReasonMessage, for a caller
// that has been told to show row values.
func (r *Refusal) Error() string {
	where := r.Col.String()
	if r.Path != "" {
		where += " at " + r.Path
	}
	return fmt.Sprintf("transform: masker %s refused %s: %s", r.Masker, where, reasonSummary(r.Reason))
}

// ReasonMessage is the wrapped error's own message, for the one caller allowed
// to print it: cmd/lazyslice's error egress under --show-row-values-in-errors,
// which is the flag whose name says what it does. It is matched structurally
// there, the way internal/core's PanicValue already is (T-0212).
func (r *Refusal) ReasonMessage() string {
	if r.Reason == nil {
		return ""
	}
	return r.Reason.Error()
}

// RefusalCode is the event code this refusal carries — an identifier from
// internal/event/catalogue.yml, never a fragment of a message. It is
// cmd/lazyslice's second, narrower discriminator for the type Error()'s
// unconditional print is granted to (T-0212 fix round, finding 3): matching
// ReasonMessage alone let any future error type that happened to declare one
// method of that name earn the same trust Error() has by construction here,
// with nobody having reviewed its text for a row value.
func (r *Refusal) RefusalCode() event.Code { return r.Code }

// reasonSummary renders a wrapped error the mask module wrote itself, and
// describes any other without quoting it: the fact that there is one and its
// type. The second half is core.PanicSummary's rule, restated here rather than
// called because internal/core imports this package and Go has no import
// cycles.
//
// The first half is the difference between a diagnosis and a shrug. Every error
// the mask module returns is documented value-free and is written by this tree,
// so "no masked value fits varchar(8)" and "masker phone can emit 44700
// distinct values" are printed in full, and only a free-form reason — a
// third-party masker's own words, an array-literal parse failure that quotes
// the literal — is withheld. For a wrapped sentinel it is the sentinel's own
// text that is printed and not the wrapper's: a masker is free to wrap
// mask.ErrNoRoom with the row in its message, and errors.Is would say yes to
// that too.
func reasonSummary(err error) string {
	if err == nil {
		return "no reason was given"
	}
	if s := maskReason(err); s != "" {
		return s
	}
	return fmt.Sprintf(
		"an error of type %T (its message is withheld because it may quote the value; "+
			"run with --show-row-values-in-errors to show it)", err)
}

// maskReason is the mask module's own refusals, every one of which names a
// category, a masker id, a type or a row count and never a value. It returns ""
// for anything else.
func maskReason(err error) string {
	var noRoom *mask.NoRoomError
	if errors.As(err, &noRoom) {
		return noRoom.Error()
	}
	var domain *mask.DomainError
	if errors.As(err, &domain) {
		return domain.Error()
	}
	for _, sentinel := range []error{
		mask.ErrUnknownMasker,
		mask.ErrNoCategory,
		mask.ErrNoRoom,
		mask.ErrKeyLength,
		mask.ErrRowCountUnknown,
		mask.ErrPassthrough,
		mask.ErrMaskerPanic,
	} {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	return ""
}

func (r *Refusal) Unwrap() error { return r.Reason }
