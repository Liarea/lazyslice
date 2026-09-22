// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// MappingFileError is returned by Read (through document.config) when a yml
// names mapping_file for a column. ADR-012 defers the mapping-file contract
// past v1, so this is a usage error the caller maps to exit 2, not a value
// nothing reads: errors.As reaches it through Read's %w wrapping.
type MappingFileError struct {
	Table  string
	Column string
}

func (e *MappingFileError) Error() string {
	return fmt.Sprintf(
		"%s.%s names mapping_file, which is not supported in this version (ADR-012); "+
			"remove the mapping_file: line for %s.%s from the yml, then use --unmask %s.%s=REASON, "+
			"or a lower --take or --cap",
		e.Table, e.Column, e.Table, e.Column, e.Table, e.Column)
}

// document is lazyslice.yml as YAML sees it (ARCHITECTURE.md section 10).
//
// It is a second shape rather than yaml tags on pipeline.Config for two
// reasons. Config is keyed by ref.TableRef and ref.ColumnRef, which have no
// YAML spelling of their own and must not grow one — a struct key would put
// `{schema: public, name: customer}` where section 10 shows `public.customer`.
// And the file is an interface: the names, the order and the nesting here are
// what a person reads and what internal/invariants parses, so they are stated
// once, in one struct, rather than scattered over the type the pipeline passes
// around.
//
// Every field is present in the encoding, including the empty ones. A key that
// disappears when it is empty is a key a reader cannot tell from one this
// version of lazyslice does not write.
type document struct {
	Version int    `yaml:"version"`
	Tool    string `yaml:"tool"`

	PlanOnly bool `yaml:"plan_only"`

	Source endpointDoc `yaml:"source"`
	Target endpointDoc `yaml:"target"`

	Root             string              `yaml:"root"`
	Take             int                 `yaml:"take"`
	Where            *string             `yaml:"where"`
	WhereFingerprint *string             `yaml:"where_fingerprint"`
	Cap              int                 `yaml:"cap"`
	Caps             map[string]int      `yaml:"caps"`
	Depth            int                 `yaml:"depth"`
	RowBudget        int64               `yaml:"row_budget"`
	MemoryBudget     string              `yaml:"memory_budget"`
	Keys             map[string][]string `yaml:"keys"`
	Skipped          []string            `yaml:"skipped"`

	SchemaFingerprint         string  `yaml:"schema_fingerprint"`
	ClassificationFingerprint string  `yaml:"classification_fingerprint"`
	SnapshotID                *string `yaml:"snapshot_id"`
	SecretFingerprint         string  `yaml:"secret_fingerprint"`

	Classify classifyDoc `yaml:"classify"`

	Columns map[string]columnDoc `yaml:"columns"`

	// Types is --allow-type-literal's own opt-outs, one entry per enum or
	// domain, keyed by the catalog's plain "schema.type" name — never quoted
	// or split, unlike Root/Caps/Keys/Skipped above: a type is never
	// decomposed back into a structured ref (pipeline.Config's own comment on
	// Types says why), so the string travels unchanged in both directions.
	Types map[string]typeAllowDoc `yaml:"types"`

	SmallDomain []string `yaml:"small_domain"`

	Plan planDoc `yaml:"plan"`
}

// endpointDoc is the source or target block: a reference, never a credential.
// password_command is a command string and never its output.
type endpointDoc struct {
	From            string  `yaml:"from"`
	Service         string  `yaml:"service"`
	Database        string  `yaml:"database"`
	Host            string  `yaml:"host"`
	Port            int     `yaml:"port"`
	User            string  `yaml:"user"`
	PasswordCommand *string `yaml:"password_command,omitempty"`
	// Params is dsn.Ref.Params: the allowlisted, non-secret transport options
	// (sslmode, sslrootcert, ...) a rerun needs to reproduce the connection
	// (docs/reviews, 2026-09-09, finding 6). Never omitted, like every other
	// field in this struct — an empty map here is a run that had none, not a
	// version of lazyslice that does not write this key.
	Params map[string]string `yaml:"params"`
}

type classifyDoc struct {
	// ExtraPatterns may add a category or raise a confidence; never lower.
	ExtraPatterns []patternDoc `yaml:"extra_patterns"`
	// PhoneRegion is --phone-region / the resolved value this run used
	// (T-0221), carried forward so a re-run needs no flag -- the same
	// pattern Root/Keys/Skipped already follow. Empty when none was ever
	// configured.
	PhoneRegion string `yaml:"phone_region"`
}

type patternDoc struct {
	Name       string `yaml:"name"`
	Category   string `yaml:"category"`
	Confidence string `yaml:"confidence"`
}

// columnDoc is one entry of the `columns:` map.
//
// masker, unique and unmask are omitted when they do not apply, because their
// presence is what they mean: a `masker:` on a column says the run masked it,
// and internal/invariants reads exactly that.
//
// MappingFile is still decoded, never encoded: ADR-012 defers the mapping_file
// contract past v1, so lazyslice never writes the key any more, but a hand-
// written or pre-ADR-012 yml naming it has to be read far enough to be refused
// by name (config, below) rather than silently ignored.
type columnDoc struct {
	Category    string     `yaml:"category"`
	Confidence  string     `yaml:"confidence"`
	Reason      string     `yaml:"reason"`
	Masker      string     `yaml:"masker,omitempty"`
	Unique      bool       `yaml:"unique,omitempty"`
	TypeFP      string     `yaml:"type,omitempty"`
	MappingFile string     `yaml:"mapping_file,omitempty"`
	Unmask      *unmaskDoc `yaml:"unmask,omitempty"`
	// Role is Decision.Role (T-0287): "given" or "family" for a person_name
	// column whose name says so, omitted for mask.RoleFull -- the zero value
	// and every other category's own, unset field -- the same way Masker is
	// omitted for a column this run did not mask.
	Role string `yaml:"role,omitempty"`
}

type unmaskDoc struct {
	Reason string `yaml:"reason"`
	By     string `yaml:"by"`
	TypeFP string `yaml:"type,omitempty"`
}

// typeAllowDoc is one entry of the `types:` map — --allow-type-literal's own
// opt-out, the same three fields unmaskDoc carries for a column.
type typeAllowDoc struct {
	Reason string `yaml:"reason"`
	By     string `yaml:"by"`
	TypeFP string `yaml:"type,omitempty"`
}

type planDoc struct {
	Tables       int            `yaml:"tables"`
	Rows         int64          `yaml:"rows"`
	Lookups      []string       `yaml:"lookups"`
	Unreachable  []string       `yaml:"unreachable"`
	Unreadable   []string       `yaml:"unreadable"`
	SCCs         [][]string     `yaml:"sccs"`
	VirtualFKs   []string       `yaml:"virtual_fks"`
	NotRecreated map[string]int `yaml:"not_recreated"`
}

// toDocument converts the in-memory config into the file's shape.
func toDocument(c *pipeline.Config) document {
	d := document{
		Version:                   c.Version,
		Tool:                      c.Tool,
		PlanOnly:                  c.PlanOnly,
		Source:                    endpoint(c.Source, c.SourceRef, c.SourceLabel, c.PasswordCommand),
		Target:                    endpoint(c.Target, c.TargetRef, c.TargetLabel, ""),
		Root:                      quoteTable(c.Root),
		Take:                      c.Take,
		Cap:                       c.Cap,
		Caps:                      tableIntMap(c.Caps),
		Depth:                     c.Depth,
		RowBudget:                 c.RowBudget,
		MemoryBudget:              FormatSize(c.MemoryBudget),
		Keys:                      tableStringsMap(c.Keys),
		Skipped:                   tableList(c.Skipped),
		SchemaFingerprint:         c.SchemaFingerprint,
		ClassificationFingerprint: c.ClassFingerprint,
		SecretFingerprint:         c.SecretFingerprint,
		Columns:                   map[string]columnDoc{},
	}
	if c.Where != "" {
		w := c.Where
		d.Where = &w
	}
	if c.WhereFingerprint != "" {
		f := c.WhereFingerprint
		d.WhereFingerprint = &f
	}
	// A --plan file carries no snapshot_id (section 10); so does a run that
	// never opened one.
	if id := string(c.SnapshotID); id != "" && !c.PlanOnly {
		d.SnapshotID = &id
	}
	for _, p := range c.ExtraPatterns {
		d.Classify.ExtraPatterns = append(d.Classify.ExtraPatterns, patternDoc{
			Name:       p.Name,
			Category:   string(p.Category),
			Confidence: confidenceName(p.Confidence),
		})
	}
	d.Classify.PhoneRegion = c.PhoneRegion
	for col, cc := range c.Columns {
		d.Columns[quoteColumn(col)] = columnDoc{
			Category:   string(cc.Category),
			Confidence: confidenceName(cc.Confidence),
			Reason:     cc.Reason,
			Masker:     string(cc.Masker),
			Unique:     cc.Unique,
			TypeFP:     cc.TypeFP,
			// mapping_file is never written (ADR-012): pipeline.ColumnConfig
			// carries no field for it.
			Unmask: unmaskOf(cc.Unmask),
			Role:   string(cc.Role),
		}
	}
	d.Types = map[string]typeAllowDoc{}
	for name, t := range c.Types {
		d.Types[name] = typeAllowDoc{Reason: t.Reason, By: t.By, TypeFP: t.TypeFP}
	}
	d.SmallDomain = columnList(c.Plan.SmallDomain)
	d.Plan = planDoc{
		Tables:       c.Plan.Tables,
		Rows:         c.Plan.Rows,
		Lookups:      tableList(c.Plan.Lookups),
		Unreachable:  tableList(c.Plan.Unreachable),
		Unreadable:   tableList(c.Plan.Unreadable),
		SCCs:         sccList(c.Plan.SCCs),
		VirtualFKs:   virtualList(c.Plan.VirtualFKs),
		NotRecreated: c.Plan.NotRecreated,
	}
	if d.Plan.NotRecreated == nil {
		d.Plan.NotRecreated = map[string]int{}
	}
	return d
}

// config converts the file back into the in-memory shape.
//
// Three of the file's blocks do not come back: `plan:`'s counts, `small_domain:`
// and `virtual_fks:` are a record of a run that is over, and nothing reads them
// as an input. Everything a re-run needs — the slice's shape, the classifier's
// priors and the opt-outs — does.
func (d document) config() (*pipeline.Config, error) {
	c := &pipeline.Config{
		Version:           d.Version,
		Tool:              d.Tool,
		PlanOnly:          d.PlanOnly,
		Source:            provenanceOf(d.Source.From),
		SourceRef:         refOf(d.Source.Host, d.Source.Port, d.Source.Database, d.Source.User, d.Source.Params),
		SourceLabel:       d.Source.Service,
		Target:            provenanceOf(d.Target.From),
		TargetRef:         refOf(d.Target.Host, d.Target.Port, d.Target.Database, d.Target.User, d.Target.Params),
		TargetLabel:       d.Target.Service,
		Take:              d.Take,
		Cap:               d.Cap,
		Depth:             d.Depth,
		RowBudget:         d.RowBudget,
		SchemaFingerprint: d.SchemaFingerprint,
		ClassFingerprint:  d.ClassificationFingerprint,
		SecretFingerprint: d.SecretFingerprint,
		Caps:              map[ref.TableRef]int{},
		Keys:              map[ref.TableRef][]string{},
		Columns:           map[ref.ColumnRef]pipeline.ColumnConfig{},
	}
	if d.Source.PasswordCommand != nil {
		c.PasswordCommand = *d.Source.PasswordCommand
	}
	if d.Where != nil {
		c.Where = *d.Where
	}
	if d.WhereFingerprint != nil {
		c.WhereFingerprint = *d.WhereFingerprint
	}
	if d.SnapshotID != nil {
		c.SnapshotID = pipeline.SnapshotID(*d.SnapshotID)
	}
	if d.MemoryBudget != "" {
		n, err := ParseSize(d.MemoryBudget)
		if err != nil {
			return nil, fmt.Errorf("memory_budget: %w", err)
		}
		c.MemoryBudget = n
	}
	if d.Root != "" {
		t, err := parseTable(d.Root)
		if err != nil {
			return nil, fmt.Errorf("root: %w", err)
		}
		c.Root = t
	}
	for name, n := range d.Caps {
		t, err := parseTable(name)
		if err != nil {
			return nil, fmt.Errorf("caps: %w", err)
		}
		c.Caps[t] = n
	}
	for name, cols := range d.Keys {
		t, err := parseTable(name)
		if err != nil {
			return nil, fmt.Errorf("keys: %w", err)
		}
		c.Keys[t] = cols
	}
	for _, name := range d.Skipped {
		t, err := parseTable(name)
		if err != nil {
			return nil, fmt.Errorf("skipped: %w", err)
		}
		c.Skipped = append(c.Skipped, t)
	}
	for _, p := range d.Classify.ExtraPatterns {
		conf, err := confidenceOf(p.Confidence)
		if err != nil {
			return nil, fmt.Errorf("classify.extra_patterns: %w", err)
		}
		c.ExtraPatterns = append(c.ExtraPatterns, pipeline.Pattern{
			Name:       p.Name,
			Category:   pipeline.Category(p.Category),
			Confidence: conf,
		})
	}
	c.PhoneRegion = d.Classify.PhoneRegion
	for name, cd := range d.Columns {
		col, err := parseColumn(name)
		if err != nil {
			return nil, fmt.Errorf("columns: %w", err)
		}
		if cd.MappingFile != "" {
			// ADR-012: mapping_file is a v1 escape hatch nothing implements yet
			// (docs/reviews/2026-09-09/REVIEW.md finding 10). Naming it is
			// refused by name rather than silently round-tripped or ignored, so
			// an operator who wrote it never gets the false confidence of a
			// clean run over a column it never touched.
			return nil, fmt.Errorf("columns: %s: %w", name, &MappingFileError{
				Table: col.Table.String(), Column: col.Column,
			})
		}
		conf, err := confidenceOf(cd.Confidence)
		if err != nil {
			return nil, fmt.Errorf("columns: %s: %w", name, err)
		}
		cc := pipeline.ColumnConfig{
			Category:   pipeline.Category(cd.Category),
			Confidence: conf,
			Reason:     cd.Reason,
			Masker:     maskerID(cd.Masker),
			Unique:     cd.Unique,
			TypeFP:     cd.TypeFP,
			Role:       mask.Role(cd.Role),
		}
		if cd.Unmask != nil {
			cc.Unmask = &pipeline.Unmask{
				Reason: cd.Unmask.Reason,
				By:     cd.Unmask.By,
				TypeFP: cd.Unmask.TypeFP,
			}
		}
		c.Columns[col] = cc
	}
	c.Types = map[string]pipeline.TypeAllow{}
	for name, td := range d.Types {
		c.Types[name] = pipeline.TypeAllow{Reason: td.Reason, By: td.By, TypeFP: td.TypeFP}
	}
	return c, nil
}

// ---------- enums ----------

var provenanceNames = map[pipeline.Provenance]string{
	pipeline.FromYml:              "yml",
	pipeline.FromEnvVar:           "env",
	pipeline.FromLibpq:            "libpq",
	pipeline.FromContainer:        "container",
	pipeline.FromStoppedContainer: "stopped_container",
	pipeline.FromCompose:          "compose",
	pipeline.FromFlag:             "flag",
}

func provenanceName(p pipeline.Provenance) string {
	if s, ok := provenanceNames[p]; ok {
		return s
	}
	return "flag"
}

func provenanceOf(s string) pipeline.Provenance {
	for p, name := range provenanceNames {
		if name == s {
			return p
		}
	}
	return pipeline.FromFlag
}

var confidenceNames = [...]string{
	pipeline.ConfNone:     "none",
	pipeline.ConfLow:      "low",
	pipeline.ConfPossible: "possible",
	pipeline.ConfLikely:   "likely",
	pipeline.ConfCertain:  "certain",
}

func confidenceName(c pipeline.Confidence) string {
	if c < 0 || int(c) >= len(confidenceNames) {
		return "none"
	}
	return confidenceNames[c]
}

func confidenceOf(s string) (pipeline.Confidence, error) {
	if s == "" {
		return pipeline.ConfNone, nil
	}
	for i, name := range confidenceNames {
		if name == s {
			return pipeline.Confidence(i), nil
		}
	}
	return pipeline.ConfNone, fmt.Errorf("%q is not a confidence: none, low, possible, likely or certain", s)
}

// ---------- identifiers ----------

// quoteIdent renders one identifier the way section 10 and PostgreSQL both
// spell it: bare when it is already lower case and needs no quoting, and
// double-quoted otherwise, with an embedded quote doubled.
//
// The quoting is not decoration. testdata/nasty.sql holds a table
// public."LegacyCustomer" and a column "EmailAddress", and a file that wrote
// them bare would name a different column when read back — while a file that
// quoted everything would not match section 10's own examples, which are all
// bare.
func quoteIdent(s string) string {
	bare := s != ""
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			bare = false
		}
		if !bare {
			break
		}
	}
	if bare {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteTable(t ref.TableRef) string {
	if t.Schema == "" && t.Name == "" {
		return ""
	}
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

func quoteColumn(c ref.ColumnRef) string {
	return quoteTable(c.Table) + "." + quoteIdent(c.Column)
}

// splitQualified splits a dotted, possibly-quoted name into its parts. A dot
// inside quotes is part of the identifier; "" inside a quoted part is one quote.
func splitQualified(s string) ([]string, error) {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' && quoted && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quoted {
		return nil, fmt.Errorf("%q has an unbalanced quote", s)
	}
	return append(parts, cur.String()), nil
}

func parseTable(s string) (ref.TableRef, error) {
	parts, err := splitQualified(s)
	if err != nil {
		return ref.TableRef{}, err
	}
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ref.TableRef{}, fmt.Errorf("%q is not a schema-qualified table name", s)
	}
	return ref.TableRef{Schema: parts[0], Name: parts[1]}, nil
}

func parseColumn(s string) (ref.ColumnRef, error) {
	parts, err := splitQualified(s)
	if err != nil {
		return ref.ColumnRef{}, err
	}
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ref.ColumnRef{}, fmt.Errorf("%q is not a schema-qualified column name", s)
	}
	return ref.ColumnRef{
		Table:  ref.TableRef{Schema: parts[0], Name: parts[1]},
		Column: parts[2],
	}, nil
}

// ---------- collections ----------

// endpoint is the source or target block. It carries the four fields of
// dsn.Ref, which holds no password, and the password *command* — a reference,
// never its output (ARCHITECTURE.md section 8).
func endpoint(p pipeline.Provenance, r dsn.Ref, label, passwordCommand string) endpointDoc {
	from := provenanceName(p)
	if r.IsZero() && label == "" {
		// No endpoint at all: a --plan run has no target, and pipeline.FromYml
		// is Provenance's zero value, so writing its name here would say the
		// yml named a database that nothing named.
		from = ""
	}
	params := r.Params
	if params == nil {
		params = map[string]string{}
	}
	return endpointDoc{
		From:            from,
		Service:         label,
		Database:        r.Database,
		Host:            r.Host,
		Port:            r.Port,
		User:            r.User,
		PasswordCommand: nilIfEmpty(passwordCommand),
		Params:          params,
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func tableList(ts []ref.TableRef) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, quoteTable(t))
	}
	sort.Strings(out)
	return out
}

func columnList(cs []ref.ColumnRef) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, quoteColumn(c))
	}
	sort.Strings(out)
	return out
}

func sccList(cs [][]ref.TableRef) [][]string {
	out := make([][]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, tableList(c))
	}
	return out
}

// virtualList renders the inferred edges as text. They are a record and never
// an input: section 3.2's mapping half ships in v1 as of T-POLY (amended
// 2026-09-08), and this list carries one entry per followed virtual edge, but
// it is never read back — a run that produced one prints it rather than
// replaying it.
func virtualList(fks []pipeline.ForeignKey) []string {
	out := make([]string, 0, len(fks))
	for _, fk := range fks {
		out = append(out, fmt.Sprintf("%s %s (%s) -> %s (%s)",
			fk.Name, quoteTable(fk.Child), strings.Join(fk.ChildCols, ", "),
			quoteTable(fk.Parent), strings.Join(fk.ParentCols, ", ")))
	}
	sort.Strings(out)
	return out
}

func tableIntMap(m map[ref.TableRef]int) map[string]int {
	out := make(map[string]int, len(m))
	for t, n := range m {
		out[quoteTable(t)] = n
	}
	return out
}

func tableStringsMap(m map[ref.TableRef][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for t, v := range m {
		out[quoteTable(t)] = v
	}
	return out
}

func unmaskOf(u *pipeline.Unmask) *unmaskDoc {
	if u == nil {
		return nil
	}
	return &unmaskDoc{Reason: u.Reason, By: u.By, TypeFP: u.TypeFP}
}
