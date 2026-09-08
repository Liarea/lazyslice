// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Polymorphic pairs (ARCHITECTURE.md §3.2, testdata/README.md trap 6).
//
// PostgreSQL knows nothing about a `<x>_type` / `<x>_id` pair, so no constraint
// can carry it and no introspection can find it. §3.2 is therefore an
// inference, and it runs in three steps: the pair is detected from the column
// names; the distinct `_type` values are sampled over a bounded, repeatable
// sample; each value is mapped to a table the Rails way (underscore and
// pluralise) or, for the `content_type_id` / `object_id` spelling, through the
// `django_content_type` row it names. Each mapping becomes a Virtual foreign
// key followed in the **parent** direction only, and everything the inference
// could not resolve is printed.
//
// Two things bound it, and both are the point rather than a detail:
//
//   - A virtual edge is followed as a parent and never as a child
//     (followsAsChild in plan.go). A parent push is PARENT_ONLY, so a row
//     reached through an inferred edge never has its own children pulled: the
//     inference can widen the slice by the parents of rows already in it, and
//     by nothing else. That is what ARCHITECTURE.md §14 means by "virtual edges
//     are parent-direction only, so enabling it later widens nothing the caps
//     do not bound" — the child steps that produce the `_type` values are
//     themselves capped and depth-limited, and the row and memory budgets are
//     checked on every pop exactly as they are for a declared edge.
//   - A pair with more than polymorphicValueCap distinct values in its sample
//     is not followed at all. A `_type` column holding free text is not a
//     discriminator, and a mapping per value would be an unbounded fan-out of
//     parent tables from one column.
//
// What inference cannot resolve keeps the v1 message. A pair whose sample
// produced no followable value at all is reported as
// `polymorphic pair detected, not followed: no constraint — public.attachments
// (owner_type, owner_id)`, with the cap named when the cap is what stopped it,
// because that reason is a threshold of ours rather than a property of the
// schema. A single value of an otherwise resolved pair that names a table this
// run cannot follow is reported the same way with the value named. A value the
// *walk* met that the sample never produced is reported the same way again,
// saying so (noteUnknownType): the sample is a sample, and a pair reported as
// resolved while an unknown number of its references were dropped is the
// silence FK-10 describes — but only a value the sample really did miss is said
// that way, because one it produced has already been reported under its own
// reason (pairPlan.sampled). A value that names no table at all is the other
// finding and goes under Unmapped, which is what research/COMPLAINTS.md FK-10 —
// the silently empty slice — exists to make impossible.

// polymorphicValueCap is how many distinct `_type` values a pair may carry and
// still be followed. Past it the column is not a discriminator, and the pair is
// reported rather than turned into that many virtual edges. It also bounds the
// unknown values one pair may report from the walk, so a `_type` column holding
// free text cannot turn the findings list into a copy of the column.
//
// It is a threshold this file introduced and ARCHITECTURE.md §3.2 does not
// state; the number is a judgement and it is recorded in this package's
// CLAUDE.md and owed an §3.2 amendment or an ADR (see the return value of
// T-POLY).
const polymorphicValueCap = 50

// polymorphicValueMaxLen bounds the length of a `_type` value in a message. The
// value is an identifier of a class rather than a row (ARCHITECTURE.md §14),
// and a column that turned out to hold something else must not be able to print
// it at length (THREAT_MODEL.md T4).
const polymorphicValueMaxLen = 64

// djangoContentTypeTable is the table Django's generic relations resolve
// through, and the three columns this reads from it.
const djangoContentTypeTable = "django_content_type"

var djangoContentTypeCols = []string{"id", "app_label", "model"}

// polymorphicPair is one detected pair.
type polymorphicPair struct {
	Table     ref.TableRef
	TypeCol   string
	IDCol     string
	ObjectCol bool // the content_type_id / object_id spelling
}

// String names the pair the way the plan reports it: the table and both
// columns, and nothing about the values in them.
func (p polymorphicPair) String() string {
	return p.Table.String() + " (" + p.TypeCol + ", " + p.IDCol + ")"
}

// polymorphicPairs finds the pairs in a table set. A pair whose id column is
// already part of a declared foreign key is not one: PostgreSQL does know about
// that edge, and the planner follows it.
func polymorphicPairs(tables []pipeline.Table, outgoing map[ref.TableRef][]pipeline.ForeignKey) []polymorphicPair {
	var out []polymorphicPair
	for _, t := range tables {
		declared := map[string]bool{}
		for _, fk := range outgoing[t.Ref] {
			for _, c := range fk.ChildCols {
				declared[c] = true
			}
		}
		names := map[string]bool{}
		for _, c := range t.Columns {
			names[c.Name] = true
		}
		for _, c := range t.Columns {
			typeCol := c.Name
			var idCol string
			object := false
			switch {
			case strings.HasSuffix(typeCol, "_type") && len(typeCol) > len("_type"):
				idCol = strings.TrimSuffix(typeCol, "_type") + "_id"
			case typeCol == "content_type_id" && names["object_id"]:
				idCol = "object_id"
				object = true
			default:
				continue
			}
			if !names[idCol] || declared[idCol] {
				continue
			}
			out = append(out, polymorphicPair{Table: t.Ref, TypeCol: typeCol, IDCol: idCol, ObjectCol: object})
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Table != out[b].Table {
			return tableRefLess(out[a].Table, out[b].Table)
		}
		return out[a].TypeCol < out[b].TypeCol
	})
	return out
}

// virtualEdge is one mapping: a `_type` value and the parent-direction edge it
// stands for. types is the referenced columns' own key encoding, which is what
// the pushed key set is built with, exactly as the declared parent step does.
type virtualEdge struct {
	value string
	fk    pipeline.ForeignKey
	types []keyType
}

// pairPlan is one detected pair the walk will follow, with the encoding of both
// its columns and one edge per mapped value in value order.
//
// sampled is every value the sample produced, mapped or not, and it is what
// tells the walk's two silences apart. A value in it that has no edge was
// already reported — as an unmapped value or as an unfollowable one — so
// meeting it again during the walk says nothing new; a value that is not in it
// is the one the sample never saw, and only that one is noteUnknownType's.
// Reporting the first kind as the second told the operator a value the sample
// did produce was one it did not, and pointed at a remedy (widen the sample)
// that would change nothing.
type pairPlan struct {
	pair     polymorphicPair
	typeType keyType
	idType   keyType
	edges    []virtualEdge
	sampled  map[string]bool
}

// inferPolymorphic is §3.2. It runs after the skip, privilege and lookup passes
// — it reads the source, so it cannot run before §3.6 has decided what this role
// may read — and before the walk, so that every edge it infers is in the plan
// the operator sees before a single row moves (§3.5).
func (p *run) inferPolymorphic(ctx context.Context) error {
	p.inferred = map[ref.TableRef][]pairPlan{}
	p.byName = map[string][]ref.TableRef{}
	p.unknownTypes = map[polymorphicPair]*unknownValues{}
	seenEdge := map[string]bool{}
	for _, t := range p.tables {
		p.byName[strings.ToLower(t.Ref.Name)] = append(p.byName[strings.ToLower(t.Ref.Name)], t.Ref)
	}

	for _, pair := range polymorphicPairs(p.tables, p.outgoing) {
		pp, res, err := p.inferPair(ctx, pair)
		if err != nil {
			return err
		}
		p.unmapped = append(p.unmapped, res.unmapped...)
		if pp == nil || len(pp.edges) == 0 {
			// Nothing was followed, so the pair keeps the v1 message — with the
			// one reason that is a threshold of ours rather than a property of
			// the schema named, because "not followed: no constraint" on a pair
			// whose values were all mappable is the one case where the sentence
			// alone is misleading.
			finding := pair.String()
			if res.overCap {
				finding += ", the sample carries more than " + strconv.Itoa(polymorphicValueCap) +
					" distinct " + pair.TypeCol + " values"
			}
			p.polymorphic = append(p.polymorphic, finding)
			continue
		}
		p.inferred[pair.Table] = append(p.inferred[pair.Table], *pp)
		for _, e := range pp.edges {
			// Two values of one pair that name the same table are one edge, so
			// the reported list carries it once. The key is the pair's
			// discriminator and the table it resolved to, which is what makes
			// two edges of one pair distinct now that the name no longer
			// restates the parent (virtualEdgeTo).
			key := e.fk.Name + " -> " + e.fk.Parent.String()
			if seenEdge[key] {
				continue
			}
			seenEdge[key] = true
			p.virtual = append(p.virtual, e.fk)
		}
		// A value of a resolved pair that names a table this run cannot follow
		// is still a finding: saying nothing about it is the silence FK-10
		// describes.
		p.polymorphic = append(p.polymorphic, res.unfollowed...)
	}
	return nil
}

// pairFindings is what inferPair found beside the walk plan: the sampled values
// that map to no table, the ones that map to a table this run will not follow,
// and whether the pair was stopped by polymorphicValueCap rather than by the
// schema. The cap is ours and the rest of the reasons are the source's, which is
// why the caller says only that one out loud.
type pairFindings struct {
	unmapped   []string
	unfollowed []string
	overCap    bool
}

// inferPair samples one pair's `_type` values and maps them. It returns the
// pair's walk plan (nil when nothing is followable) and what the sample found
// that no edge covers.
func (p *run) inferPair(ctx context.Context, pair polymorphicPair) (*pairPlan, pairFindings, error) {
	var found pairFindings
	if !p.inScope[pair.Table] || p.lookups[pair.Table] {
		// A table the run does not walk has no rows to infer from: a skipped or
		// unreadable table is SchemaOnly, and a lookup is copied whole and never
		// has its parents pulled (§3's lookup rule).
		return nil, found, nil
	}
	tbl := p.byRef[pair.Table]
	if tbl == nil {
		return nil, found, nil
	}
	cols := columnsByName(tbl)
	typeCol, ok := cols[pair.TypeCol]
	if !ok {
		return nil, found, nil
	}
	idCol, ok := cols[pair.IDCol]
	if !ok {
		return nil, found, nil
	}
	typeType, idType := typeOf(typeCol), typeOf(idCol)
	if !polymorphicIDKind(idType.kind) {
		// The id column travels as a key, so it has to be one of the kinds
		// keyset.go carries without a cast back. A `_id` of any other type is
		// not the Rails or Django shape and is reported rather than guessed at.
		return nil, found, nil
	}

	values, err := p.sampleTypeValues(ctx, pair, typeType)
	if err != nil {
		return nil, found, err
	}
	if len(values) > polymorphicValueCap {
		found.overCap = true
		return nil, found, nil
	}
	if len(values) == 0 {
		return nil, found, nil
	}

	pp := &pairPlan{pair: pair, typeType: typeType, idType: idType, sampled: make(map[string]bool, len(values))}
	for _, v := range values {
		pp.sampled[v] = true
	}
	for _, v := range values {
		parent, ok, err := p.mapTypeValue(ctx, pair, v)
		if err != nil {
			return nil, found, err
		}
		if !ok {
			found.unmapped = append(found.unmapped,
				pair.Table.String()+"."+pair.TypeCol+" value "+showValue(v))
			continue
		}
		edge, ok := p.virtualEdgeTo(pair, v, parent, idType)
		if !ok {
			found.unfollowed = append(found.unfollowed,
				pair.String()+" where "+pair.TypeCol+" = "+showValue(v))
			continue
		}
		pp.edges = append(pp.edges, edge)
	}
	return pp, found, nil
}

// polymorphicIDKind says whether a `_id` column's encoding is one a virtual
// edge may travel under: an integer, a uuid or a text key. bpchar is excluded
// because its comparison strips padding, and kindOther because its join carries
// a cast back to a type a polymorphic id never has.
func polymorphicIDKind(k keyKind) bool {
	return k == kindInt || k == kindUUID || k == kindText
}

// sampleTypeValues reads the distinct `_type` values over a bounded, repeatable
// sample of the table, in the column's own order. It is §3.2's "distinct `_type`
// values are sampled", and it is a sample rather than the whole column because
// no statement of this package reads a table whole (THREAT_MODEL.md T9).
func (p *run) sampleTypeValues(ctx context.Context, pair polymorphicPair, ty keyType) ([]string, error) {
	sql := p.distinctSQL(p.byRef[pair.Table], []string{pair.TypeCol}, []keyType{ty})
	ks, err := p.readKeys(ctx, sql, []keyType{ty})
	if err != nil {
		return nil, fmt.Errorf("plan: sampling %s.%s: %w", pair.Table, pair.TypeCol, err)
	}
	var out []string
	ks.forEach(func(tuple []keyValue) {
		if v, ok := renderTypeValue(ty, tuple[0]); ok {
			out = append(out, v)
		}
	})
	return out, nil
}

// distinctSQL is the statement one distinct-value sample takes: §3.4's
// TABLESAMPLE where it can be taken, and the ordered bounded prefix on the two
// relations where it cannot — a partitioned table, and one nothing has analysed,
// where a fraction of an unknown row count is a full scan wearing a probe's
// name (sql.go, and the same rule probePseudoKey applies).
//
// The prefix orders by the table's identity, which is resolved before §3.2 runs
// (resolveIdentities, in run.plan). That is index-backed for a primary key and
// a unique index; for a probed pseudo-key on a table nothing has analysed it is
// a sort, which is the price of a reproducible sample on the one relation that
// can offer nothing cheaper.
func (p *run) distinctSQL(tbl *pipeline.Table, cols []string, types []keyType) string {
	if tbl.Partitioned || tbl.ApproxRows <= 0 {
		return distinctPrefixSQL(tbl.Ref, cols, types, p.ids[tbl.Ref].Columns, probeSampleRows)
	}
	num, den := samplePercent(tbl.ApproxRows)
	return distinctSampleSQL(tbl.Ref, cols, types, num, den, probeSampleRows)
}

// renderTypeValue is the text form of one `_type` value, and it is both the map
// key the walk splits on and the text a message prints. A uuid discriminator is
// refused: it names no class in either framework.
func renderTypeValue(ty keyType, v keyValue) (string, bool) {
	switch ty.kind {
	case kindInt:
		return strconv.FormatInt(v.i, 10), true
	case kindText, kindBpchar, kindOther:
		return v.s, true
	case kindUUID:
		return "", false
	}
	return "", false
}

// showValue renders a `_type` value for a message: quoted, escaped and bounded.
// §14 admits the value into the log because it identifies a class and not a row;
// the bound is there for the column that turned out to hold something else.
func showValue(v string) string {
	if len(v) > polymorphicValueMaxLen {
		v = v[:polymorphicValueMaxLen] + "..."
	}
	return strconv.Quote(v)
}

// mapTypeValue maps one sampled `_type` value to a table. The Django spelling
// resolves through the django_content_type row the value names; every other
// pair is the Rails one, where the value is a class name and the table is its
// underscored, pluralised form.
func (p *run) mapTypeValue(ctx context.Context, pair polymorphicPair, value string) (ref.TableRef, bool, error) {
	if pair.ObjectCol {
		return p.mapDjangoContentType(ctx, pair, value)
	}
	t, ok := p.tableNamed(railsCandidates(value), pair.Table.Schema)
	return t, ok, nil
}

// tableNamed resolves the first candidate name that names exactly one planned
// table, preferring the schema the pair itself is in. A candidate that names a
// table in two other schemas resolves to neither: a guess between them would put
// rows in the slice for a reason the plan cannot state.
func (p *run) tableNamed(candidates []string, prefer string) (ref.TableRef, bool) {
	for _, name := range candidates {
		matches := p.byName[name]
		for _, m := range matches {
			if m.Schema == prefer {
				return m, true
			}
		}
		if len(matches) == 1 {
			return matches[0], true
		}
	}
	return ref.TableRef{}, false
}

// railsCandidates is the table names a Rails `_type` value could be, §3.2's
// rule first.
//
// §3.2 states the rule as "underscore and pluralise", and that form is tried
// before anything else, for the value as written and for the value without its
// module path — a class name is stored with one, and `Admin::User` may be
// either the `admin_users` or the `users` table. An earlier version put the raw
// and un-pluralised forms first, which inverted the rule: `User` resolved to a
// `user` table in preference to `users`.
//
// The un-pluralised forms are still tried, after it, and that is a deliberate
// deviation from §3.2 rather than an oversight: a `_type` column that holds the
// table name itself is the shape testdata/nasty.sql trap 6 carries and the
// shape hand-rolled polymorphism takes, and dropping the fallback would leave
// both unmapped. It costs a wrong edge only where a schema carries a table
// named exactly after the value and no pluralised one, and it is owed a §3.2
// amendment (see the return value of T-POLY).
func railsCandidates(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	forms := []string{value, demodulize(value)}
	for _, form := range forms {
		add(pluralize(underscore(form)))
	}
	for _, form := range forms {
		add(strings.ToLower(form))
		add(underscore(form))
	}
	return out
}

// demodulize drops a class name's module path: `Admin::User` is `User`.
func demodulize(s string) string {
	if i := strings.LastIndex(s, "::"); i >= 0 {
		return s[i+2:]
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// underscore is ActiveSupport's: `::` and `/` become `_`, a word boundary
// inside a CamelCase name becomes `_`, and the result is lower case.
func underscore(s string) string {
	s = strings.ReplaceAll(s, "::", "/")
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == '/' || r == ' ' || r == '-':
			b.WriteByte('_')
		case unicode.IsUpper(r):
			prev := rune(0)
			if i > 0 {
				prev = runes[i-1]
			}
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if i > 0 && prev != '_' && prev != '/' &&
				(unicode.IsLower(prev) || unicode.IsDigit(prev) || unicode.IsLower(next)) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// irregularPlurals is the short list an English suffix rule gets wrong and that
// a real schema actually carries. `person` is the one that matters: Rails'
// canonical polymorphic example stores `Person` against a `people` table.
var irregularPlurals = map[string]string{
	"person": "people",
	"man":    "men",
	"woman":  "women",
	"child":  "children",
	"foot":   "feet",
	"tooth":  "teeth",
	"goose":  "geese",
	"mouse":  "mice",
	"ox":     "oxen",
}

// pluralize is the suffix half of Rails' `tableize`, over the last word of an
// underscored name. It is deliberately small: a candidate that is wrong names no
// table, and a value that names no table is reported rather than guessed at.
func pluralize(s string) string {
	if s == "" {
		return s
	}
	head, tail := "", s
	if i := strings.LastIndex(s, "_"); i >= 0 {
		head, tail = s[:i+1], s[i+1:]
	}
	if p, ok := irregularPlurals[tail]; ok {
		return head + p
	}
	switch {
	case strings.HasSuffix(tail, "s"), strings.HasSuffix(tail, "x"), strings.HasSuffix(tail, "z"),
		strings.HasSuffix(tail, "ch"), strings.HasSuffix(tail, "sh"):
		return head + tail + "es"
	case len(tail) > 1 && strings.HasSuffix(tail, "y") && !isVowel(tail[len(tail)-2]):
		return head + tail[:len(tail)-1] + "ies"
	default:
		return head + tail + "s"
	}
}

func isVowel(b byte) bool { return strings.IndexByte("aeiou", b) >= 0 }

// mapDjangoContentType resolves a `content_type_id` value through the
// django_content_type row it names: the row carries the app label and the model
// name, and Django's default table for a model is `<app_label>_<model>`.
func (p *run) mapDjangoContentType(ctx context.Context, pair polymorphicPair, value string) (ref.TableRef, bool, error) {
	id, ok := parseContentTypeID(value)
	if !ok {
		return ref.TableRef{}, false, nil
	}
	types, err := p.djangoContentTypes(ctx)
	if err != nil {
		return ref.TableRef{}, false, err
	}
	names, ok := types[id]
	if !ok {
		return ref.TableRef{}, false, nil
	}
	t, ok := p.tableNamed(djangoCandidates(names[0], names[1]), pair.Table.Schema)
	return t, ok, nil
}

// parseContentTypeID reads a content_type_id back out of its rendered form. A
// value that is not one names no content type, which is an unmapped value and
// not a failure of the run.
func parseContentTypeID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil
}

// djangoCandidates is the table name one django_content_type row stands for,
// and it is §3.2's rule and nothing else: Django's default table for a model is
// `<app_label>_<model>`. The bare model name is deliberately not a fallback —
// an `auth`/`user` content type would bind to any app's `public.user` — and a
// model with an explicit db_table therefore resolves to no table and is
// reported as an unmapped value, which is the finding this feature already has
// for a name it cannot place.
func djangoCandidates(appLabel, model string) []string {
	appLabel, model = strings.ToLower(strings.TrimSpace(appLabel)), strings.ToLower(strings.TrimSpace(model))
	if model == "" || appLabel == "" {
		return nil
	}
	return []string{appLabel + "_" + model}
}

// djangoContentTypes reads django_content_type once per run, over a bounded
// prefix like every other probe. It answers nothing when the table is not in the
// source, which is every schema that is not Django's: the pair is then reported
// as one inference could not resolve.
func (p *run) djangoContentTypes(ctx context.Context) (map[int64][2]string, error) {
	if p.djangoLoaded {
		return p.django, nil
	}
	p.djangoLoaded = true
	p.django = map[int64][2]string{}

	var src *pipeline.Table
	for i := range p.tables {
		if p.tables[i].Ref.Name == djangoContentTypeTable && p.inScope[p.tables[i].Ref] {
			src = p.byRef[p.tables[i].Ref]
			break
		}
	}
	if src == nil {
		return p.django, nil
	}
	types, err := typesFor(src, djangoContentTypeCols)
	if err != nil {
		// A table of that name without those columns is not Django's.
		return p.django, nil //nolint:nilerr // an absent column is "not Django", not a failure
	}
	if types[0].kind != kindInt || !textKind(types[1].kind) || !textKind(types[2].kind) {
		return p.django, nil
	}
	sql := p.distinctSQL(src, djangoContentTypeCols, types)
	ks, err := p.readKeys(ctx, sql, types)
	if err != nil {
		return nil, fmt.Errorf("plan: reading %s: %w", src.Ref, err)
	}
	ks.forEach(func(tuple []keyValue) {
		p.django[tuple[0].i] = [2]string{tuple[1].s, tuple[2].s}
	})
	return p.django, nil
}

func textKind(k keyKind) bool { return k == kindText || k == kindBpchar || k == kindOther }

// virtualEdgeTo builds the edge one mapping stands for. The referenced columns
// are the parent's primary key, because that is what a `_id` column of either
// framework holds, and the edge is built only when the key is one column of the
// same encoding as the `_id` column: two different key spaces joined by
// inference would select rows for a reason nothing can state.
func (p *run) virtualEdgeTo(pair polymorphicPair, value string, parent ref.TableRef, idType keyType) (virtualEdge, bool) {
	if !p.inScope[parent] {
		return virtualEdge{}, false
	}
	tbl := p.byRef[parent]
	if tbl == nil || len(tbl.PK) != 1 {
		return virtualEdge{}, false
	}
	types, err := typesFor(tbl, tbl.PK)
	if err != nil || types[0].kind != idType.kind {
		return virtualEdge{}, false
	}
	return virtualEdge{
		value: value,
		types: types,
		fk: pipeline.ForeignKey{
			// The name is one identifier — the discriminator column the
			// inference read — and never the `_type` value it resolved *from*.
			// internal/emit copies Plan.Virtual into lazyslice.yml's
			// `virtual_fks:`, and ARCHITECTURE.md §10 says every value in that
			// file is an identifier, a count, a fingerprint or a flag value with
			// literals withheld; a sampled column value is none of those. The
			// value stays where §14 does admit it — the log, through Step.Why
			// and through the unmapped and unfollowable findings.
			//
			// It names the discriminator and stops there because emit's
			// virtualList renders `Name Child (cols) -> Parent (cols)`: a name
			// that carried the parent too printed the arrow clause twice, and
			// the one thing the reconstructed edge cannot say on its own is
			// which column the discriminator was. Two edges of one pair are
			// therefore told apart by name *and* parent, which is the key
			// inferPolymorphic dedupes on.
			Name:       pair.Table.String() + "." + pair.TypeCol,
			Child:      pair.Table,
			ChildCols:  []string{pair.IDCol},
			Parent:     parent,
			ParentCols: []string{tbl.PK[0]},
			Virtual:    true,
		},
	}, true
}

// virtualParents is §3.2's half of the parent step: the rows a table's
// polymorphic pairs reference, pushed PARENT_ONLY like every other parent.
//
// One read per pair per chunk answers every one of its values at once: the
// statement is the ordinary parent read with both columns of the pair in its
// select list, so the discriminator is applied here rather than as a predicate
// carrying a value into the statement. That is what keeps the read inside
// mapKeysSQL's registered shape, and it is also the fix for the bug this
// feature is most likely to have — Greenmask's #396, where a polymorphic
// predicate that lost its type guard silently selected almost nothing.
func (p *run) virtualParents(ctx context.Context, it item, fresh *keys) ([]item, error) {
	plans := p.inferred[it.table]
	if len(plans) == 0 {
		return nil, nil
	}
	var out []item
	id := p.ids[it.table]
	for _, pp := range plans {
		outCols := []string{pp.pair.TypeCol, pp.pair.IDCol}
		outTypes := []keyType{pp.typeType, pp.idType}
		for _, ch := range fresh.Chunks(chunkSize) {
			sql := mapKeysSQL(it.table, id.Columns, id.types, outCols, outTypes, outCols)
			got, err := p.readKeys(ctx, sql, outTypes, chunkArgs(ch, len(id.types))...)
			if err != nil {
				return nil, fmt.Errorf("plan: reading the polymorphic pair %s: %w", pp.pair, err)
			}
			if got.Len() == 0 {
				continue
			}
			byValue := make(map[string]*keys, len(pp.edges))
			for _, e := range pp.edges {
				byValue[e.value] = newKeys(e.types)
			}
			got.forEach(func(tuple []keyValue) {
				v, ok := renderTypeValue(pp.typeType, tuple[0])
				if !ok {
					return
				}
				if ks := byValue[v]; ks != nil {
					ks.add(tuple[1:2])
					return
				}
				if pp.sampled[v] {
					// The sample did produce this one, and inferPair already
					// reported it — as a value that maps to no table, or as one
					// whose table this run will not follow. Recording it again
					// would print the same value twice under two reasons, one of
					// them false.
					return
				}
				// A value the sample never produced. Dropping it here is
				// research/COMPLAINTS.md FK-10 exactly — a slice quietly
				// narrower than the operator was told — so it is recorded and
				// reported beside the values inference could not resolve.
				p.noteUnknownType(pp.pair, v)
			})
			pushed, err := p.pushVirtual(ctx, it, pp, byValue)
			if err != nil {
				return nil, err
			}
			out = append(out, pushed...)
		}
	}
	return out, nil
}

// pushVirtual turns one chunk's split-by-value referenced keys into queue items.
func (p *run) pushVirtual(ctx context.Context, it item, pp pairPlan, byValue map[string]*keys) ([]item, error) {
	var out []item
	for _, e := range pp.edges {
		refs := byValue[e.value]
		if refs == nil || refs.Len() == 0 {
			continue
		}
		if !p.inScope[e.fk.Parent] || p.lookups[e.fk.Parent] {
			continue
		}
		pk, err := p.virtualIdentityKeys(ctx, e.fk.Parent, e.fk.ParentCols, e.types, refs)
		if err != nil {
			return nil, err
		}
		pending := p.subtract(e.fk.Parent, pk)
		if pending.Len() == 0 {
			continue
		}
		p.noteWhy(e.fk.Parent, "parent of "+it.table.String()+" via the polymorphic pair "+
			it.table.String()+"."+pp.pair.IDCol+" where "+pp.pair.TypeCol+" = "+showValue(e.value))
		out = append(out, item{table: e.fk.Parent, keys: pending, mode: pipeline.ParentOnly, depth: it.depth})
	}
	return out, nil
}

// virtualIdentityKeys turns the values a polymorphic `_id` column holds into
// the parent's own identity keys, and it always asks the parent, where
// identityKeys short-circuits when the referenced columns are already the
// identity.
//
// That short-circuit is sound for a declared foreign key and only for one: the
// constraint is what guarantees the referenced row exists, so the referenced
// value *is* an identity key of a row. Behind an inferred edge there is no
// constraint, so a dangling `_type`/`_id` pair — testdata/nasty.sql ships one on
// purpose — would otherwise enter the parent's key set as a row that does not
// exist. The cost of that is not theoretical: the phantom key inflates
// Estimate.Rows and the printed step count, extract returns one row fewer than
// the plan promised, and internal/verify refuses the finished run at exit 7
// with the target already dropped and loaded — a planner defect reported as a
// load failure.
//
// The read is the registered plan.map_keys shape, the same statement
// identityKeys sends when the referenced columns are not the identity, and it
// returns only the keys of rows that are there.
func (p *run) virtualIdentityKeys(
	ctx context.Context, parent ref.TableRef, cols []string, types []keyType, refs *keys,
) (*keys, error) {
	id := p.ids[parent]
	out := newKeys(id.types)
	for _, ch := range refs.Chunks(chunkSize) {
		sql := mapKeysSQL(parent, cols, types, id.Columns, id.types, nil)
		got, err := p.readKeys(ctx, sql, id.types, chunkArgs(ch, len(types))...)
		if err != nil {
			return nil, fmt.Errorf("plan: translating %s keys behind a virtual edge: %w", parent, err)
		}
		got.forEach(out.add)
	}
	return out, nil
}

// unknownValues is one pair's `_type` values that the walk saw and the sample
// did not, bounded by the same cap the sample itself takes.
type unknownValues struct {
	values map[string]bool
	more   bool // values arrived past the cap
}

// noteUnknownType records a `_type` value the walk met that no edge covers.
//
// The sample is a sample: a value that lives only in the rows it did not read
// has no edge, and the row's parent is not followed. What must not happen is
// that it goes unsaid — the plan would report the pair as fully resolved while
// an unknown number of its references were dropped, which is the silently
// narrow slice research/COMPLAINTS.md FK-10 describes and this feature exists to
// make impossible.
func (p *run) noteUnknownType(pair polymorphicPair, value string) {
	if p.unknownTypes == nil {
		p.unknownTypes = map[polymorphicPair]*unknownValues{}
	}
	u := p.unknownTypes[pair]
	if u == nil {
		u = &unknownValues{values: map[string]bool{}}
		p.unknownTypes[pair] = u
	}
	if u.values[value] {
		return
	}
	if len(u.values) >= polymorphicValueCap {
		u.more = true
		return
	}
	u.values[value] = true
}

// unknownFindings is what noteUnknownType collected, in pair order and then in
// value order, spelled like the unfollowable-value finding beside it: both are
// "detected, not followed", and this one says which half of the pair went
// unanswered and why.
func (p *run) unknownFindings() []string {
	pairs := make([]polymorphicPair, 0, len(p.unknownTypes))
	for pair := range p.unknownTypes {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a].Table != pairs[b].Table {
			return tableRefLess(pairs[a].Table, pairs[b].Table)
		}
		return pairs[a].TypeCol < pairs[b].TypeCol
	})

	var out []string
	for _, pair := range pairs {
		u := p.unknownTypes[pair]
		values := make([]string, 0, len(u.values))
		for v := range u.values {
			values = append(values, v)
		}
		sort.Strings(values)
		for _, v := range values {
			out = append(out, pair.String()+" where "+pair.TypeCol+" = "+showValue(v)+
				", a value the sample did not produce")
		}
		if u.more {
			out = append(out, pair.String()+" carries more than "+strconv.Itoa(polymorphicValueCap)+
				" further "+pair.TypeCol+" values the sample did not produce")
		}
	}
	return out
}
