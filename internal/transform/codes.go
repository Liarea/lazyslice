// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
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
	// Reason is the wrapped error. It is not rendered into the event: the
	// catalogue's template names the column and the masker only.
	Reason error
}

func (r *Refusal) Error() string {
	where := r.Col.String()
	if r.Path != "" {
		where += " at " + r.Path
	}
	return fmt.Sprintf("transform: masker %s refused %s: %v", r.Masker, where, r.Reason)
}

func (r *Refusal) Unwrap() error { return r.Reason }
