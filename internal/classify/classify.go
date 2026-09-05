// Package classify decides what every column holds: names, types, validated
// samples, dictionaries, and a reason string for each decision
// (ARCHITECTURE.md section 4).
//
// It is pure. It never issues SQL: samples arrive through pipeline.Sampler,
// taken at introspect. That is what lets the whole classifier be tested against
// a fixture without a database, and it is why the rule pack can be changed with
// confidence.
//
// It is biased to recall. After the neighbouring-column rule and FK propagation
// have run, "possible" and above is masked; the failure mode is mask more, never
// less. There is no ML gate and no cloud model over values.
//
// The classifier is not pluggable, by design and not by omission (ADR-006): a
// pluggable classifier is a supported way to see less personal data. The rule
// pack is an embedded YAML file in this package and contributions to it are pull
// requests. lazyslice.yml may add a pattern or raise a confidence; it can never
// lower or remove one.
//
// Scaffold status: no-op. Classify returns pipeline.ErrNotImplemented; the rule
// pack, the validators, the dictionaries and reasons.go arrive with the
// classification task.
package classify

import (
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type classifier struct{}

// New returns the rule-pack classifier.
func New() pipeline.Classifier { return classifier{} }

var _ pipeline.Classifier = classifier{}

func (classifier) Classify(_ *pipeline.Schema, _ pipeline.Sampler, _ *pipeline.Config) (*pipeline.Classification, error) {
	return nil, fmt.Errorf("classify: %w", pipeline.ErrNotImplemented)
}
