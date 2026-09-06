// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Precision and recall on Pagila, against a truth table labelled by hand.
//
// The schema below is testdata/pagila/pagila-schema.sql's 15 base tables,
// recorded as internal/introspect would report them, with the seven payment
// leaf partitions left out because they repeat the root's six columns and would
// do nothing but inflate the true-negative count. No sample values are recorded:
// this measures the name and type signals alone, which is the harder half —
// research/HARD_PROBLEMS.md §3.2's column-level detectors are name-based and
// average macro-F1 0.61 to 0.87, and the number this test prints is the one to
// compare against.
//
// The labels are a judgement about Pagila's own schema, not a general rule:
// city.city and country.country are lookup tables of world place names rather
// than anyone's address, and film.description is product-catalogue prose. All
// three are false positives here and all three are ones lazyslice should keep
// making: masking a lookup costs a lookup, and ARCHITECTURE.md §10 shows
// film.description carrying exactly this opt-out.

// pagilaTruth is the hand-labelled answer: true means the column holds personal
// data about an identifiable person, and lazyslice must mask it.
var pagilaTruth = map[string]bool{
	"public.actor.actor_id":            false,
	"public.actor.first_name":          true,
	"public.actor.last_name":           true,
	"public.actor.last_update":         false,
	"public.address.address_id":        false,
	"public.address.address":           true,
	"public.address.address2":          true,
	"public.address.district":          true,
	"public.address.city_id":           false,
	"public.address.postal_code":       true,
	"public.address.phone":             true,
	"public.address.last_update":       false,
	"public.category.category_id":      false,
	"public.category.name":             false,
	"public.category.last_update":      false,
	"public.city.city_id":              false,
	"public.city.city":                 false,
	"public.city.country_id":           false,
	"public.city.last_update":          false,
	"public.country.country_id":        false,
	"public.country.country":           false,
	"public.country.last_update":       false,
	"public.customer.customer_id":      false,
	"public.customer.store_id":         false,
	"public.customer.first_name":       true,
	"public.customer.last_name":        true,
	"public.customer.email":            true,
	"public.customer.address_id":       false,
	"public.customer.activebool":       false,
	"public.customer.create_date":      false,
	"public.customer.last_update":      false,
	"public.customer.active":           false,
	"public.film.film_id":              false,
	"public.film.title":                false,
	"public.film.description":          false,
	"public.film.release_year":         false,
	"public.film.language_id":          false,
	"public.film.original_language_id": false,
	"public.film.rental_duration":      false,
	"public.film.rental_rate":          false,
	"public.film.length":               false,
	"public.film.replacement_cost":     false,
	"public.film.rating":               false,
	"public.film.last_update":          false,
	"public.film.special_features":     false,
	"public.film.fulltext":             false,
	"public.film_actor.actor_id":       false,
	"public.film_actor.film_id":        false,
	"public.film_actor.last_update":    false,
	"public.film_category.film_id":     false,
	"public.film_category.category_id": false,
	"public.film_category.last_update": false,
	"public.inventory.inventory_id":    false,
	"public.inventory.film_id":         false,
	"public.inventory.store_id":        false,
	"public.inventory.last_update":     false,
	"public.language.language_id":      false,
	"public.language.name":             false,
	"public.language.last_update":      false,
	"public.payment.payment_id":        false,
	"public.payment.customer_id":       false,
	"public.payment.staff_id":          false,
	"public.payment.rental_id":         false,
	"public.payment.amount":            false,
	"public.payment.payment_date":      false,
	"public.rental.rental_id":          false,
	"public.rental.rental_date":        false,
	"public.rental.inventory_id":       false,
	"public.rental.customer_id":        false,
	"public.rental.return_date":        false,
	"public.rental.staff_id":           false,
	"public.rental.last_update":        false,
	"public.staff.staff_id":            false,
	"public.staff.first_name":          true,
	"public.staff.last_name":           true,
	"public.staff.address_id":          false,
	"public.staff.email":               true,
	"public.staff.store_id":            false,
	"public.staff.active":              false,
	"public.staff.username":            true,
	"public.staff.password":            true,
	"public.staff.last_update":         false,
	"public.staff.picture":             true,
	"public.store.store_id":            false,
	"public.store.manager_staff_id":    false,
	"public.store.address_id":          false,
	"public.store.last_update":         false,
}

func pagilaSchema() *pipeline.Schema {
	var (
		actor    = ref.TableRef{Schema: "public", Name: "actor"}
		address  = ref.TableRef{Schema: "public", Name: "address"}
		category = ref.TableRef{Schema: "public", Name: "category"}
		city     = ref.TableRef{Schema: "public", Name: "city"}
		country  = ref.TableRef{Schema: "public", Name: "country"}
		customer = ref.TableRef{Schema: "public", Name: "customer"}
		film     = ref.TableRef{Schema: "public", Name: "film"}
		filmA    = ref.TableRef{Schema: "public", Name: "film_actor"}
		filmC    = ref.TableRef{Schema: "public", Name: "film_category"}
		inv      = ref.TableRef{Schema: "public", Name: "inventory"}
		lang     = ref.TableRef{Schema: "public", Name: "language"}
		payment  = ref.TableRef{Schema: "public", Name: "payment"}
		rental   = ref.TableRef{Schema: "public", Name: "rental"}
		staff    = ref.TableRef{Schema: "public", Name: "staff"}
		store    = ref.TableRef{Schema: "public", Name: "store"}
	)
	paymentTbl := tt("public", "payment", []string{"payment_id", "payment_date"},
		tc("payment_id", "integer"),
		tc("customer_id", "integer"),
		tc("staff_id", "integer"),
		tc("rental_id", "integer"),
		tc("amount", "numeric(5,2)"),
		tc("payment_date", "timestamp with time zone"),
	)
	paymentTbl.Partitioned = true
	paymentTbl.PartitionKey = []string{"payment_date"}

	filmTbl := tt("public", "film", []string{"film_id"},
		tc("film_id", "integer"),
		tc("title", "text"),
		tc("description", "text"),
		tc("release_year", "public.year"),
		tc("language_id", "integer"),
		tc("original_language_id", "integer"),
		tc("rental_duration", "smallint"),
		tc("rental_rate", "numeric(4,2)"),
		tc("length", "smallint"),
		tc("replacement_cost", "numeric(5,2)"),
		tc("rating", "public.mpaa_rating"),
		tc("last_update", "timestamp with time zone"),
		tc("special_features", "text[]"),
		tc("fulltext", "tsvector"),
	)
	filmTbl.Columns[3].Domain = "public.year"

	return &pipeline.Schema{
		ServerVersion: 160000,
		Schemas:       []string{"public"},
		Enums:         map[string][]string{"public.mpaa_rating": {"G", "PG", "PG-13", "R", "NC-17"}},
		Domains: []pipeline.NamedDef{
			{Name: "public.year", Def: "CREATE DOMAIN public.year AS integer CONSTRAINT year_check CHECK (((VALUE >= 1901) AND (VALUE <= 2155)))"},
		},
		Tables: []pipeline.Table{
			tt("public", "actor", []string{"actor_id"},
				tc("actor_id", "integer"), tc("first_name", "text"), tc("last_name", "text"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "address", []string{"address_id"},
				tc("address_id", "integer"), tc("address", "text"), tc("address2", "text"),
				tc("district", "text"), tc("city_id", "integer"), tc("postal_code", "text"),
				tc("phone", "text"), tc("last_update", "timestamp with time zone")),
			tt("public", "category", []string{"category_id"},
				tc("category_id", "integer"), tc("name", "text"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "city", []string{"city_id"},
				tc("city_id", "integer"), tc("city", "text"), tc("country_id", "integer"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "country", []string{"country_id"},
				tc("country_id", "integer"), tc("country", "text"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "customer", []string{"customer_id"},
				tc("customer_id", "integer"), tc("store_id", "integer"), tc("first_name", "text"),
				tc("last_name", "text"), tc("email", "text"), tc("address_id", "integer"),
				tc("activebool", "boolean"), tc("create_date", "date"),
				tc("last_update", "timestamp with time zone"), tc("active", "integer")),
			filmTbl,
			tt("public", "film_actor", []string{"actor_id", "film_id"},
				tc("actor_id", "integer"), tc("film_id", "integer"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "film_category", []string{"film_id", "category_id"},
				tc("film_id", "integer"), tc("category_id", "integer"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "inventory", []string{"inventory_id"},
				tc("inventory_id", "integer"), tc("film_id", "integer"), tc("store_id", "integer"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "language", []string{"language_id"},
				tc("language_id", "integer"), tc("name", "character(20)"),
				tc("last_update", "timestamp with time zone")),
			paymentTbl,
			tt("public", "rental", []string{"rental_id"},
				tc("rental_id", "integer"), tc("rental_date", "timestamp with time zone"),
				tc("inventory_id", "integer"), tc("customer_id", "integer"),
				tc("return_date", "timestamp with time zone"), tc("staff_id", "integer"),
				tc("last_update", "timestamp with time zone")),
			tt("public", "staff", []string{"staff_id"},
				tc("staff_id", "integer"), tc("first_name", "text"), tc("last_name", "text"),
				tc("address_id", "integer"), tc("email", "text"), tc("store_id", "integer"),
				tc("active", "boolean"), tc("username", "text"), tc("password", "text"),
				tc("last_update", "timestamp with time zone"), tc("picture", "bytea")),
			tt("public", "store", []string{"store_id"},
				tc("store_id", "integer"), tc("manager_staff_id", "integer"),
				tc("address_id", "integer"), tc("last_update", "timestamp with time zone")),
		},
		FKs: []pipeline.ForeignKey{
			fk("address_city_id_fkey", address, []string{"city_id"}, city, []string{"city_id"}),
			fk("city_country_id_fkey", city, []string{"country_id"}, country, []string{"country_id"}),
			fk("customer_address_id_fkey", customer, []string{"address_id"}, address, []string{"address_id"}),
			fk("customer_store_id_fkey", customer, []string{"store_id"}, store, []string{"store_id"}),
			fk("film_language_id_fkey", film, []string{"language_id"}, lang, []string{"language_id"}),
			fk("film_original_language_id_fkey", film, []string{"original_language_id"}, lang, []string{"language_id"}),
			fk("film_actor_actor_id_fkey", filmA, []string{"actor_id"}, actor, []string{"actor_id"}),
			fk("film_actor_film_id_fkey", filmA, []string{"film_id"}, film, []string{"film_id"}),
			fk("film_category_category_id_fkey", filmC, []string{"category_id"}, category, []string{"category_id"}),
			fk("film_category_film_id_fkey", filmC, []string{"film_id"}, film, []string{"film_id"}),
			fk("inventory_film_id_fkey", inv, []string{"film_id"}, film, []string{"film_id"}),
			fk("inventory_store_id_fkey", inv, []string{"store_id"}, store, []string{"store_id"}),
			fk("payment_customer_id_fkey", payment, []string{"customer_id"}, customer, []string{"customer_id"}),
			fk("payment_rental_id_fkey", payment, []string{"rental_id"}, rental, []string{"rental_id"}),
			fk("payment_staff_id_fkey", payment, []string{"staff_id"}, staff, []string{"staff_id"}),
			fk("rental_customer_id_fkey", rental, []string{"customer_id"}, customer, []string{"customer_id"}),
			fk("rental_inventory_id_fkey", rental, []string{"inventory_id"}, inv, []string{"inventory_id"}),
			fk("rental_staff_id_fkey", rental, []string{"staff_id"}, staff, []string{"staff_id"}),
			fk("staff_address_id_fkey", staff, []string{"address_id"}, address, []string{"address_id"}),
			fk("staff_store_id_fkey", staff, []string{"store_id"}, store, []string{"store_id"}),
			fk("store_address_id_fkey", store, []string{"address_id"}, address, []string{"address_id"}),
			fk("store_manager_staff_id_fkey", store, []string{"manager_staff_id"}, staff, []string{"staff_id"}),
		},
		Fingerprint: "recorded-pagila-fixture",
	}
}

// scoreboard is a confusion matrix over a labelled column set.
type scoreboard struct {
	tp, fp, tn, fn int
	falsePositives []string
	falseNegatives []string
}

func (s scoreboard) precision() float64 {
	if s.tp+s.fp == 0 {
		return 1
	}
	return float64(s.tp) / float64(s.tp+s.fp)
}

func (s scoreboard) recall() float64 {
	if s.tp+s.fn == 0 {
		return 1
	}
	return float64(s.tp) / float64(s.tp+s.fn)
}

func (s scoreboard) f1() float64 {
	p, r := s.precision(), s.recall()
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

// print writes the confusion matrix the way a reader wants to see it: the
// counts, the rates, and then every column the classifier and the label
// disagreed about, by name.
func (s scoreboard) print(t *testing.T, title string) {
	t.Helper()
	sort.Strings(s.falsePositives)
	sort.Strings(s.falseNegatives)
	t.Log(title)
	t.Log("                     labelled personal   labelled not")
	t.Logf("  masked                       %5d          %5d", s.tp, s.fp)
	t.Logf("  copied                       %5d          %5d", s.fn, s.tn)
	t.Logf("  precision %.3f   recall %.3f   F1 %.3f   n=%d",
		s.precision(), s.recall(), s.f1(), s.tp+s.fp+s.tn+s.fn)
	for _, c := range s.falseNegatives {
		t.Log("  false negative (personal data copied): " + c)
	}
	for _, c := range s.falsePositives {
		t.Log("  false positive (masked anyway): " + c)
	}
}

// score compares a classification against a truth table.
func score(t *testing.T, cls *pipeline.Classification, truth map[string]bool) scoreboard {
	t.Helper()
	var s scoreboard
	seen := map[string]bool{}
	for c, d := range cls.Decisions {
		want, labelled := truth[c.String()]
		if !labelled {
			t.Errorf("%s is not in the truth table; label it rather than leaving it out", c)
			continue
		}
		seen[c.String()] = true
		switch {
		case want && d.Masked:
			s.tp++
		case want && !d.Masked:
			s.fn++
			s.falseNegatives = append(s.falseNegatives, c.String())
		case !want && d.Masked:
			s.fp++
			s.falsePositives = append(s.falsePositives, c.String())
		default:
			s.tn++
		}
	}
	for c := range truth {
		if !seen[c] {
			t.Errorf("%s is in the truth table but the classifier decided nothing for it", c)
		}
	}
	return s
}

// TestPagilaPrecisionAndRecall is the number this package is judged on. Recall
// is the one that must not move: a false negative is cleartext in the target and
// exit 0, which is THREAT_MODEL.md T1. Precision is a cost, and the floor below
// is deliberately loose — a rule that raised it by dropping a name pattern would
// be trading the thing that matters for the thing that does not.
func TestPagilaPrecisionAndRecall(t *testing.T) {
	t.Parallel()
	cls, err := New().Classify(pagilaSchema(), mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	s := score(t, cls, pagilaTruth)
	s.print(t, "Pagila, name and type signals only:")

	if s.recall() < 1.0 {
		t.Errorf("recall = %.3f, want 1.0: every false negative is cleartext in the target (THREAT_MODEL.md T1)", s.recall())
	}
	if s.precision() < 0.70 {
		t.Errorf("precision = %.3f, want at least 0.70", s.precision())
	}
}

// pagilaSamples are the values introspect would hand the classifier for the two
// pagila columns T-0054 is about, as pgx decodes them: a `timestamp with time
// zone` arrives as a time.Time, and a `tsvector` — a type pgx has no codec for —
// arrives as the text form Postgres prints.
//
// They are here because the fixture above records no samples at all, and the
// bug this file now guards against is invisible without them: it is a *value*
// signal firing on a column whose type cannot hold what the category's masker
// emits.
func pagilaSamples() mapSampler {
	s := mapSampler{}
	stamps := anyOf(
		time.Date(2017, 2, 15, 9, 34, 33, 0, time.UTC),
		time.Date(2017, 2, 15, 9, 34, 33, 0, time.UTC),
		time.Date(2017, 2, 15, 10, 2, 19, 0, time.UTC),
		time.Date(2020, 12, 23, 7, 12, 45, 0, time.UTC),
		time.Date(2022, 6, 1, 18, 45, 30, 0, time.UTC),
	)
	for _, t := range pagilaSchema().Tables {
		for _, c := range t.Columns {
			if c.Name == "last_update" {
				s[col(t.Ref, c.Name)] = stamps
			}
		}
	}
	// film.fulltext as Postgres prints a tsvector: lexeme:position pairs, which
	// carry digits and words and so satisfy the address validator's "mixed
	// digits and words" shape on every row.
	s[col(ref.TableRef{Schema: "public", Name: "film"}, "fulltext")] = anyOf(
		"'academi':1 'battl':15 'canadian':20 'dinosaur':2 'epistl':7",
		"'ace':1 'administr':9 'ancient':19 'astound':4 'car':17 'china':20",
		"'adapt':1 'astound':4 'baloon':19 'factori':20 'holes':1",
		"'affair':1 'boat':4 'documentari':7 'shark':14 'sumo':20",
		"'african':1 'chase':11 'dentist':16 'egg':4 'forens':10 'mad':20",
	)
	return s
}

// The blocker T-0054 was opened for, at the place it starts.
//
// Every `last_update` in pagila is a `timestamp with time zone` whose text form
// ("2017-02-15T09:34:33Z") has no space and no "@", mixes character classes and
// clears the entropy threshold, so the credential validator fires on 100% of the
// samples. `film.fulltext` is a tsvector whose printed lexemes are digits and
// words, which is the address validator's shape. Neither category's masker can
// write into either column: the run used to reach `internal/transform` and die
// at exit 7 with rows already moved.
//
// ARCHITECTURE.md §4's accepted-types gate now runs over a value signal as it
// always did over a name signal, so neither column can be decided above `low`
// for a category its type cannot hold, and the reason says which conflict it
// was.
func TestPagilaValueSignalsRespectAcceptedTypes(t *testing.T) {
	t.Parallel()
	cls, err := New().Classify(pagilaSchema(), pagilaSamples(), nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	conflicts := 0
	for c, d := range cls.Decisions {
		if c.Column != "last_update" {
			continue
		}
		if d.Masked || d.Confidence > pipeline.ConfLow {
			t.Errorf("%s is %s at %v (%s); a timestamp holds no category whose masker emits text",
				c, d.Category, d.Confidence, d.Reason)
		}
		if d.Category == pipeline.CatNone {
			continue
		}
		// A category recorded at low is the value signal that fired, kept as a
		// record with the conflict named rather than dropped silently.
		want := "is not an accepted type for " + string(d.Category)
		if !strings.Contains(d.Reason, want) {
			t.Errorf("%s is %s at low and its reason does not say why it was not masked: %s", c, d.Category, d.Reason)
			continue
		}
		conflicts++
	}
	if conflicts == 0 {
		t.Error("no last_update column recorded a type conflict: the samples no longer trip the validator, so this test is no longer testing the gate")
	}

	fulltext := col(ref.TableRef{Schema: "public", Name: "film"}, "fulltext")
	d, ok := cls.Decisions[fulltext]
	if !ok {
		t.Fatal("film.fulltext has no decision")
	}
	if d.Category == pipeline.CatAddress {
		t.Errorf("film.fulltext is address (%s); a tsvector holds no street", d.Reason)
	}
	// It is masked, but by its type and not by its values: a tsvector carries
	// the lexemes of the text it was derived from, which may itself be masked.
	if d.Category != CatDerivedText || !d.Masked {
		t.Errorf("film.fulltext is %s masked=%v, want %s masked (%s)", d.Category, d.Masked, CatDerivedText, d.Reason)
	}
}
