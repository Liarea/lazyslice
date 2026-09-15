// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// The recreated-DDL literal rule (ARCHITECTURE.md §11.1, amended 2026-09-14,
// T-0134; THREAT_MODEL.md T1).
//
// §11.1 recreates a column's DEFAULT, its CHECK constraints and a generated
// column's expression as the catalog's own text, verbatim. A string literal
// inside one of them therefore crosses into the target exactly as a row value
// does, and until this check existed it crossed unexamined: the 2026-09-09
// review put DEFAULT 'ddl.canary@example.org' on an email column, watched the
// *row* be masked and the *default* survive into the target's pg_attrdef, and
// the run exited 0. A later INSERT that takes the default materialises the
// original address again, so a scan of the rows cannot establish that the
// database artefact holds no sensitive literal (finding 5,
// docs/reviews/2026-09-09/evidence/ddl_default.log).
//
// The rule this implements, stated once:
//
//   - A literal in a **masked column's DEFAULT** is masked through that
//     column's own masker — the same key, category and generator its rows go
//     through, so the default stays a *working* default whose value is the one
//     a row holding that literal would have. Nothing is dropped: the
//     application's first INSERT is the point of the tool (§11.1).
//   - A literal in a **CHECK or generated expression on a masked column** is
//     never rewritten: the predicate and the derivation are the application's,
//     and a masked literal inside one changes what the database accepts or
//     computes. A literal there that a *strong* validator hits is refused at
//     exit 13 naming the object, because leaving it is the leak and rewriting
//     it is not ours to do. A literal no strong validator hits is left alone —
//     `CHECK (status IN ('active','banned'))` on a masked column is a closed
//     value list the masker already honours (mask.Constraints.Checks), and
//     refusing on it would refuse most real schemas over labels that are not
//     personal data.
//   - A literal in **any of the three on an unmasked column** that a strong
//     validator hits is exit 12 at plan naming the object, with the --unmask
//     escape: the operator says in writing, per column and with a reason, that
//     the literal in that column's DDL is not a person's. A column that already
//     carries such an opt-out is not asked twice.
//
// A fourth class was added by the 2026-09-15 red team (checkTypeLiterals): an
// enum's labels and a domain's DEFAULT and CHECK, which §11.1 also recreates
// verbatim and which this pass did not read at all. Neither can be rewritten,
// so both are exit 13 with no rewrite arm — and the escape is
// `--allow-type-literal TYPE=REASON`, §8's per-column opt-out spelt for the one object
// class that is not a column (the T-REDFIX review's fourth finding; before it
// the refusal named --skip-table, which cannot clear it).
//
// "Strong" is email, phone, payment card, IBAN and national_id — the five value
// shapes a parser decides rather than a dictionary guesses (internal/textsig,
// and see strongValidators below for what the last two cost and why they are
// admissible). The narrowness is deliberate and is recorded in THREAT_MODEL.md
// T1: a name or an address inside a CHECK is not refused, because the
// false-positive rate of those signals over SQL fragments is what would make
// this check unusable, and internal/verify's catalog pass is the second look at
// the artefact this one admits.
//
// Where it runs: after checkUniqueDomain, because the masker a default is
// rewritten with must be the one the *rows* are masked with, and the equality
// group and unique-index rules overwrite Decision.Masker (unique.go,
// equality.go). It is still before assemble, so no key has left this stage and
// nothing in the target has been touched — which is all §11.1 requires of a
// refusal raised "at plan".

// strongValidator is one of the value shapes a parser decides. The names
// are the classifier's category names, so a refusal says `email` and not a
// value (THREAT_MODEL.md T4).
type strongValidator struct {
	name string
	ok   func(string) bool
}

// The set was three until the 2026-09-15 red team's A20, which put
// "Alice Anderson, 42 Elm St, SSN 123-45-6789, IBAN GB33BUKB20201555555555"
// into a CHECK on an unmasked column and watched it cross into the target under
// exit 0. THREAT_MODEL.md T1 already stated the name-and-address half of that
// miss and gave the reason — a CHECK is full of English words, so a dictionary
// heuristic over one would refuse ordinary schemas — and that reason does not
// reach the other two: a US Social Security number and a UK National Insurance
// number are strict patterns and an IBAN is a mod-97 checksum, none of them a
// guess. So national_id and the IBAN half of financial_account join the set,
// and person_name and address stay out for exactly the reason the document
// gives.
//
// IBAN could not have joined before this task: textsig.ValidIBAN matched any
// fifteen-to-thirty-four-character run of letters and digits that cleared the
// mod-97 check, which five of pagila's own film titles do. It now requires the
// two ISO 13616 check digits in positions three and four, which every real IBAN
// has and an all-caps title does not.
var strongValidators = []strongValidator{
	{name: string(pipeline.CatEmail), ok: textsig.ValidEmail},
	{name: string(pipeline.CatPhone), ok: textsig.ValidPhone},
	{name: string(pipeline.CatFinancial), ok: textsig.ValidLuhn},
	{name: string(pipeline.CatFinancial), ok: textsig.ValidIBAN},
	{name: string(pipeline.CatNationalID), ok: textsig.ValidNationalID},
}

// strongHit is the first strong validator a literal matches, or "".
//
// A pattern operand is never a hit: it is the right-hand side of a LIKE or a
// regex operator, so it is a shape and not a value. testdata/nasty.sql's
// CHECK ("EmailAddress" LIKE '%@%.%') is why this is a rule and not a footnote
// -- % is an ordinary atext character, net/mail parses '%@%.%' as a valid
// address, and the whole fixture refused at exit 12 the first time this check
// ran.
func strongHit(lit pipeline.Literal) string {
	if lit.Pattern {
		return ""
	}
	s := strings.TrimSpace(lit.Text)
	if s == "" {
		return ""
	}
	for _, v := range strongValidators {
		if v.ok(s) {
			return v.name
		}
	}
	return ""
}

// markerTable is the bookkeeping table of §11.2, which internal/load/ddl does
// not recreate and this check therefore does not read. The name is repeated
// rather than imported: internal/plan may not import another stage package
// (internal/CLAUDE.md), and internal/load/CLAUDE.md records the same constant
// on its side.
const markerTable = "lazyslice_meta"

// checkDDLLiterals runs the rule above over every object §11.1 recreates.
//
// The tables are every non-partition table of the schema, in (schema, name)
// order — not the in-scope ones. §11.1 recreates the DDL of a SchemaOnly table
// too, so its default reaches the target whether or not a row does, and a check
// that skipped it would let --skip-table carry the literal through.
func (p *run) checkDDLLiterals() error {
	if p.cls == nil {
		return nil
	}
	if err := p.checkTypeLiterals(); err != nil {
		return err
	}
	for i := range p.tables {
		t := &p.tables[i]
		if t.Ref.Name == markerTable {
			continue
		}
		if err := p.tableDDLLiterals(t); err != nil {
			return err
		}
	}
	return nil
}

// checkTypeLiterals is the object classes §11.1 recreates that belong to a
// *type* rather than to a table: an enum's labels and a domain's whole CREATE
// statement, which carries its DEFAULT and its CHECK.
//
// It is the 2026-09-15 red team's A4b, A11 and A12, and each of the three is
// one catalog table to the side of where the 2026-09-14 amendment (T-0134)
// looked. internal/load/ddl writes `CREATE TYPE ... AS ENUM ('a','b')` and
// replays a domain's definition verbatim, so a value in either crosses into the
// target exactly as a column DEFAULT does — and this pass read pg_attrdef,
// pg_constraint and pg_index only, so `CREATE TYPE assignee AS ENUM
// ('unassigned','enum.canary@bigcorp.com','+1-415-555-0199')` and
// `CREATE DOMAIN tenant_d AS text DEFAULT 'domdefault.canary@bigcorp.com'` both
// exited 0 with the values in the target's catalog.
//
// Everything here refuses at 13 and nothing here is rewritten, which is the one
// place this file's three-way rule (mask / refuse at 13 / refuse at 12) collapses
// to one outcome:
//
//   - An enum label cannot be rewritten. Every row of every column of that type
//     references the label *by value*, so a masked label either breaks the
//     column or silently remaps rows — and there is no --unmask that makes a
//     label a person's or not a person's, because a label is not a column.
//   - A domain's DEFAULT and CHECK belong to the type. The columns declared
//     over the domain may be masked under different categories, or not masked
//     at all, so there is no single masker whose output would be the right
//     replacement — which is exactly the argument columnDefault makes for a
//     column whose shape it declines.
//
// The refusal names the type and the position of the label, never the label
// text (THREAT_MODEL.md T4).
//
// **The escape is --allow-type-literal TYPE=REASON, and it had to be built for this**
// (the T-REDFIX review's fourth finding). The first version of this check named
// --skip-table, which cannot clear it by any route: --skip-table drops a table
// to *schema only*, so its DDL — and every type that DDL names — is still
// recreated, and nothing in internal/core prunes Schema.Enums or
// Schema.Domains while internal/load/ddl's typeOrder recreates every one of
// them regardless. A source schema with a single enum label or domain
// definition that a strong validator hit was therefore permanently unrunnable,
// under a refusal that told the operator to try a flag with no effect on it.
// The opt-out carries a reason for the same purpose --unmask does, it is
// resolved against the source's own type names by internal/core (so an opt-out
// that could never apply is exit 2 rather than silence), and
// internal/verify's catalog pass honours the same list through
// Plan.AllowedTypeLiterals — an escape one end of the pipeline grants and the other
// refuses is not an escape.
func (p *run) checkTypeLiterals() error {
	names := make([]string, 0, len(p.schema.Enums))
	for name := range p.schema.Enums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if p.allowedTypeLiteral(name) {
			continue
		}
		for i, label := range p.schema.Enums[name] {
			// A label is the value itself and not an expression, so it is
			// judged directly rather than scanned for literals inside it.
			if hit := strongHit(pipeline.Literal{Text: label}); hit != "" {
				return p.refuseTypeLiteral(name, "label "+strconv.Itoa(i+1), hit)
			}
		}
	}
	domains := append([]pipeline.NamedDef(nil), p.schema.Domains...)
	sort.Slice(domains, func(a, b int) bool { return domains[a].Name < domains[b].Name })
	for _, d := range domains {
		if p.allowedTypeLiteral(d.Name) {
			continue
		}
		for _, lit := range pipeline.Literals(d.Def) {
			if hit := strongHit(lit); hit != "" {
				return p.refuseTypeLiteral(d.Name, "its definition", hit)
			}
		}
	}
	return nil
}

// allowedTypeLiteral reports that the operator has said in writing, with a reason,
// that this type's recreated definition carries no person's value
// (--allow-type-literal TYPE=REASON). The name is the catalog's qualified one, which
// is what internal/core resolves the flag to.
func (p *run) allowedTypeLiteral(name string) bool {
	_, ok := p.req.AllowTypeLiterals[name]
	return ok
}

// allowedTypeNames is the opt-outs that name a type this schema actually
// carries, in name order, for Plan.AllowedTypeLiterals. A name the schema does not
// carry is already exit 2 in internal/core; filtering again here keeps the plan
// from telling internal/verify to exempt an object nobody named.
func (p *run) allowedTypeNames() []string {
	if len(p.req.AllowTypeLiterals) == 0 {
		return nil
	}
	out := make([]string, 0, len(p.req.AllowTypeLiterals))
	for name := range p.req.AllowTypeLiterals {
		if _, ok := p.schema.Enums[name]; ok {
			out = append(out, name)
			continue
		}
		for _, d := range p.schema.Domains {
			if d.Name == name {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// refuseTypeLiteral is exit 13 naming a type. typeRef splits the qualified name
// the catalog recorded so that the refusal reads like every other one.
func (p *run) refuseTypeLiteral(typeName, where, hit string) error {
	t := typeRef(typeName)
	r := refuse(CodeTypeLiteral, exitSchema, t,
		fmt.Sprintf("type %s carries a literal in %s that parses as %s and cannot be rewritten, "+
			"so the target's schema would hold it as the source wrote it", typeName, where, hit),
		event.Args{
			event.ArgTable:  typeName,
			event.ArgColumn: where,
			event.ArgReason: "a literal in " + where + " parses as " + hit +
				" and lazyslice cannot mask a type in place: --allow-type-literal " +
				typeName + "=REASON says it is not a person's",
		})
	r.Column = where
	return r
}

// typeRef splits a catalog-qualified type name into a ref.TableRef so that a
// refusal about a type carries the same two identifiers a refusal about a table
// does. A name with no schema part keeps the whole string as the name.
func typeRef(name string) ref.TableRef {
	if i := strings.LastIndex(name, "."); i > 0 {
		return ref.TableRef{Schema: name[:i], Name: name[i+1:]}
	}
	return ref.TableRef{Name: name}
}

func (p *run) tableDDLLiterals(t *pipeline.Table) error {
	masked := map[string]pipeline.Decision{}
	for _, col := range t.Columns {
		d, ok := p.cls.Decisions[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
		if ok && d.Masked {
			masked[col.Name] = d
		}
	}

	for j := range t.Columns {
		col := t.Columns[j]
		d, isMasked := masked[col.Name]
		if originalDefault(col) != "" {
			if err := p.columnDefault(t, j, col, d, isMasked); err != nil {
				return err
			}
		}
		if col.Generated != "" {
			if err := p.fixedExpression(t, col.Name, col.Generated, isMasked,
				[]string{col.Name}, "the generated expression on"); err != nil {
				return err
			}
		}
	}

	cons := append([]pipeline.Constraint(nil), t.Constraints...)
	sort.Slice(cons, func(a, b int) bool { return cons[a].Name < cons[b].Name })
	for _, con := range cons {
		if con.Kind != 'c' && con.Kind != 'x' {
			continue
		}
		named := namedColumns(con.Def, t)
		if err := p.fixedExpression(t, con.Name, con.Def,
			anyMasked(named, masked), named, "the constraint"); err != nil {
			return err
		}
	}
	return nil
}

// originalDefault is the catalog's own text of a column's default: Default on a
// column this stage has not rewritten, and Column.DefaultOriginal on one it has.
//
// Everything in this file reads the default through here, so the check is
// **idempotent** on the *pipeline.Schema it is given. The rewrite below writes
// the masked text back over Column.Default, and a second Plan over the same
// in-memory schema — internal/tui re-plans against a cached one, and
// plan_integration_test.go plans twice to compare two plans — would otherwise
// mask the already-masked literal and produce mask(mask(x)): a different default
// on the second plan than on the first, and a DEFAULT whose value is not the one
// a row holding the source's literal gets.
func originalDefault(col pipeline.Column) string {
	if col.DefaultOriginal != "" {
		return col.DefaultOriginal
	}
	return col.Default
}

// columnDefault masks a masked column's default in place, or refuses.
//
// The rewrite is written back through p.byRef, which points into p.schema —
// p.tables holds copies, and internal/load/ddl generates the target's DDL from
// the schema. That is the one thing this stage mutates outside the
// classification, and internal/plan/CLAUDE.md records it.
func (p *run) columnDefault(
	t *pipeline.Table,
	index int,
	col pipeline.Column,
	d pipeline.Decision,
	isMasked bool,
) error {
	def := originalDefault(col)
	lits := pipeline.Literals(def)
	if len(lits) == 0 {
		return nil
	}
	if !isMasked {
		return p.refuseUnmaskedLiteral(t, col.Name, lits, []string{col.Name}, "the default on")
	}

	c, judged := p.constraintsOf(col)
	shapeRewritable := judged && p.defaultIsRewritable(col)

	// No key yet is not the same finding as "cannot be rewritten": every shape
	// this stage declines below is a property of the column and stays wrong
	// however this run is invoked, but a nil Key *with* KeyPending set is
	// internal/core's keyBeforePlan reporting that a plan-only run has none to
	// resolve without creating one (T-0161). KeyPending is read explicitly
	// here rather than inferred from Key == nil alone (2026-09-14 review,
	// finding 1): a writing run always reaches here with a key — core
	// resolves it in full before planStage runs, and sets KeyPending false —
	// so a nil Key on such a run is drift, not the plan-only state, and falls
	// through to the refusal below instead of silently skipping the column.
	// This branch is what a `lazyslice plan` with no lazyslice.secret and no
	// $LAZYSLICE_SECRET takes, and refusing it would make planning impossible
	// before a secret exists at all. The column is reported instead, in the
	// order this pass visits it, so the operator can act.
	if shapeRewritable && p.req.Key == nil && p.req.KeyPending {
		p.pendingKeyDefaults = append(p.pendingKeyDefaults, t.Ref.String()+"."+col.Name)
		return nil
	}
	if !shapeRewritable || p.req.Key == nil {
		// Nothing here can produce a value for this column — either the shape
		// declines it, or (KeyPending false, drift guard) a key was expected
		// and is missing — so a literal that is plainly personal data cannot
		// be shipped and cannot be replaced.
		for _, lit := range lits {
			if hit := strongHit(lit); hit != "" {
				return p.refuseNotRewritable(t, col.Name, "the default on", hit)
			}
		}
		return nil
	}
	c.Unique = uniqueColumn(t, col.Name) || d.UniqueIndex

	var failed error
	out, ok := pipeline.RewriteLiterals(def, func(lit pipeline.Literal) (string, bool) {
		if failed != nil || lit.Pattern || strings.TrimSpace(lit.Text) == "" {
			return "", false
		}
		res, err := mask.Apply(*p.req.Key, mask.Category(d.Category), d.Masker,
			mask.Value{Text: lit.Text}, c)
		if err != nil {
			failed = err
			return "", false
		}
		return res.Out.Text, true
	})
	if failed != nil || !ok {
		for _, lit := range lits {
			if hit := strongHit(lit); hit != "" {
				return p.refuseNotRewritable(t, col.Name, "the default on", hit)
			}
		}
		return nil
	}
	p.byRef[t.Ref].Columns[index].Default = out
	p.byRef[t.Ref].Columns[index].DefaultOriginal = def
	t.Columns[index].Default = out
	t.Columns[index].DefaultOriginal = def
	return nil
}

// uniqueColumn reports that the column sits alone under a unique index or is the
// whole primary key — ARCHITECTURE.md §5's "the column is under a unique index",
// which mask.Constraints.Unique carries and which changes what a generator
// emits (an email masker on a unique column emits a hash-derived suffix).
//
// It is a **copy** of internal/transform's function of the same name
// (constraints.go), for the reason constraintsOf above is a copy of transform's
// reduction: a stage package may not import another (internal/CLAUDE.md), and a
// default masked under different Constraints than the column's rows is masked
// under is exactly the defect this rewrite exists to avoid — the default would
// come from the non-unique generator while every row came from the unique one.
// transform ORs Decision.UniqueIndex on top of it (transform.go) and so does the
// caller, because classify's field asks a narrower question than this one does
// (classify.go's own note) and neither answer subsumes the other. The two
// spellings are a cross-package contract with nothing but this comment holding
// them together; **T-0164** owes them a shared home, as T-0162 owes the literal
// scanner one.
func uniqueColumn(t *pipeline.Table, name string) bool {
	if t == nil {
		return false
	}
	if len(t.PK) == 1 && t.PK[0] == name {
		return true
	}
	for _, idx := range t.Indexes {
		if idx.Unique && !idx.Partial && !idx.Expression && len(idx.Columns) == 1 && idx.Columns[0] == name {
			return true
		}
	}
	return false
}

// defaultIsRewritable reports whether a masked column's default is one this
// stage may substitute literals into at all.
//
// Three shapes are declined, and each is declined because a masked literal in
// it would be wrong rather than merely different:
//
//   - An array column. §5 masks an array element-wise; the default of a text[]
//     column is one literal holding the whole array's text form, and masking it
//     as a scalar produces something that is not an array literal at all
//     (testdata/regressions/009 is the row-side version of that mistake).
//   - A json, jsonb or hstore column. §4 replaces a document whole, per leaf;
//     the same argument.
//   - A default calling nextval. Its literal is a sequence name cast to
//     regclass, and a masked sequence name is a relation the target does not
//     have — a CREATE TABLE that fails after every table has been dropped.
//
// Each is a refusal rather than a pass when the literal is plainly personal
// data, which is what the caller does with the false.
func (p *run) defaultIsRewritable(col pipeline.Column) bool {
	name := strings.ToLower(strings.TrimSpace(col.TypeName))
	if strings.HasSuffix(name, "[]") {
		return false
	}
	switch mask.BareTypeName(mask.StripTypmod(name)) {
	case "json", "jsonb", "hstore":
		return false
	}
	return !strings.Contains(strings.ToLower(originalDefault(col)), "nextval(")
}

// fixedExpression is the CHECK and generated-expression half: never rewritten,
// refused at 13 on a masked column and at 12 on an unmasked one.
func (p *run) fixedExpression(
	t *pipeline.Table,
	object, def string,
	onMasked bool,
	named []string,
	what string,
) error {
	lits := pipeline.Literals(def)
	if len(lits) == 0 {
		return nil
	}
	if !onMasked {
		return p.refuseUnmaskedLiteral(t, object, lits, named, what)
	}
	for _, lit := range lits {
		if hit := strongHit(lit); hit != "" {
			return p.refuseNotRewritable(t, object, what, hit)
		}
	}
	return nil
}

// namedColumns is the columns of the table a deparsed definition names, in the
// table's own column order. It is a token match over the catalog's own text,
// which quotes a name that needs quoting and writes a bare one otherwise, so
// both spellings are looked for.
func namedColumns(def string, t *pipeline.Table) []string {
	var out []string
	for _, col := range t.Columns {
		quoted := `"` + strings.ReplaceAll(col.Name, `"`, `""`) + `"`
		if strings.Contains(def, quoted) || containsWord(def, col.Name) {
			out = append(out, col.Name)
		}
	}
	return out
}

// anyMasked reports whether any of the named columns is masked.
func anyMasked(named []string, masked map[string]pipeline.Decision) bool {
	for _, name := range named {
		if _, ok := masked[name]; ok {
			return true
		}
	}
	return false
}

// containsWord reports whether def carries name as a whole identifier.
func containsWord(def, name string) bool {
	if name == "" {
		return false
	}
	for at := 0; ; {
		i := strings.Index(def[at:], name)
		if i < 0 {
			return false
		}
		i += at
		before := i == 0 || !identByte(def[i-1])
		end := i + len(name)
		after := end >= len(def) || !identByte(def[end])
		if before && after {
			return true
		}
		at = i + 1
	}
}

func identByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// refuseUnmaskedLiteral is exit 12: a strong validator hit in the DDL of a
// column this run does not mask. The escape is the per-column --unmask, which
// is the only place in the tool an operator says in writing, with a reason,
// that a column's contents are not a person's.
func (p *run) refuseUnmaskedLiteral(
	t *pipeline.Table,
	object string,
	lits []pipeline.Literal,
	named []string,
	what string,
) error {
	escape := object
	for _, name := range named {
		if p.optedOut(t.Ref, name) {
			return nil
		}
		if escape == object && name != "" {
			escape = name
		}
	}
	for _, lit := range lits {
		hit := strongHit(lit)
		if hit == "" {
			continue
		}
		r := refuse(CodeDDLLiteral, exitPlan, t.Ref,
			fmt.Sprintf("%s %s.%s carries a literal that parses as %s, and the column is not masked, "+
				"so the literal would be recreated in the target as it stands",
				what, t.Ref, object, hit),
			event.Args{
				event.ArgTable:  t.Ref.String(),
				event.ArgColumn: object,
				event.ArgReason: "a literal in " + what + " this column parses as " + hit +
					": --unmask " + t.Ref.String() + "." + escape + "=REASON says it is not a person's",
			})
		r.Column = object
		return r
	}
	return nil
}

// optedOut reports whether the operator has already said in writing that this
// column's contents are not a person's. An opt-out that classify did not honour
// — its type fingerprint moved (THREAT_MODEL.md T3) — is not one, because
// Decision.Source is then still the classifier's.
func (p *run) optedOut(t ref.TableRef, column string) bool {
	d, ok := p.cls.Decisions[ref.ColumnRef{Table: t, Column: column}]
	if !ok {
		return false
	}
	return d.Source == pipeline.ByFlagUnmask || d.Source == pipeline.ByYmlUnmask
}

// refuseNotRewritable is exit 13: the object is inside the data boundary, the
// literal is plainly personal data, and this stage may not rewrite it.
func (p *run) refuseNotRewritable(t *pipeline.Table, object, what, hit string) error {
	r := refuse(CodeLiteralNotRewritable, exitSchema, t.Ref,
		fmt.Sprintf("%s %s.%s carries a literal that parses as %s and cannot be rewritten, "+
			"so the target's schema would hold it as the source wrote it",
			what, t.Ref, object, hit),
		event.Args{
			event.ArgTable:  t.Ref.String(),
			event.ArgColumn: object,
			event.ArgReason: "a literal in " + what + " this column parses as " + hit +
				" and lazyslice cannot mask it in place",
		})
	r.Column = object
	return r
}
