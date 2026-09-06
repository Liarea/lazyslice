// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"math"
	"regexp"
	"strings"
)

// Domain arithmetic saturates at math.MaxInt64 rather than wrapping. A wrapped
// domain reads as a small one, and a small one is what the planner refuses on,
// so wrapping would turn a 2⁹⁶ generator into a refusal or, worse, a negative
// number that compares below d_required in the wrong direction.

func satMul(a, b int64) int64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

func satPow(base int64, exp int) int64 {
	out := int64(1)
	for i := 0; i < exp; i++ {
		out = satMul(out, base)
		if out == math.MaxInt64 {
			return out
		}
	}
	return out
}

// The type families Constraints.TypeTag carries, in the names
// internal/classify/types.go uses. mask cannot import that package — it is a
// separate module and internal/ — so the spellings are repeated here, and the
// rule pack's accepts: lists are where the two are read side by side.
const (
	famText      = "text"
	famVarchar   = "varchar"
	famBpchar    = "bpchar"
	famCitext    = "citext"
	famBoolean   = "boolean"
	famInteger   = "integer"
	famBigint    = "bigint"
	famNumeric   = "numeric"
	famFloat     = "float"
	famDate      = "date"
	famTimestamp = "timestamp"
	famUUID      = "uuid"
	famInet      = "inet"
	famCIDR      = "cidr"
	famMacaddr   = "macaddr"
	famBytea     = "bytea"
	famJSON      = "json"
	famJSONB     = "jsonb"
	famHstore    = "hstore"
	famEnum      = "enum"
)

// numericFamily reports whether the family holds a number rather than a string.
func numericFamily(tag string) bool {
	switch tag {
	case famInteger, famBigint, famNumeric, famFloat:
		return true
	default:
		return false
	}
}

// room is the number of bytes the column will accept, or 0 for unbounded.
func room(c Constraints) int {
	if c.MaxLen < 0 {
		return 0
	}
	return c.MaxLen
}

// fits reports whether s is short enough for the column.
func fits(s string, c Constraints) bool {
	return c.MaxLen <= 0 || len(s) <= c.MaxLen
}

// digitBudget is how many decimal digits the column can hold: its length for a
// string, the type's width for an integer.
func digitBudget(c Constraints, want int) int {
	n := want
	switch c.TypeTag {
	case famInteger:
		n = min(n, 9)
	case famBigint, famNumeric, famFloat:
		n = min(n, 18)
	default:
		if c.MaxLen > 0 {
			n = min(n, c.MaxLen)
		}
	}
	if c.MaxLen > 0 {
		n = min(n, c.MaxLen)
	}
	return n
}

// The CHECK shapes this module parses: an explicit list of allowed values,
// which is the shape that actually bounds a domain and that a masked value must
// land inside or fail the load, and an exact length. Anything else in
// Constraints.Checks is left alone and mask/CLAUDE.md records that: an unparsed
// CHECK can make a load fail loudly, which is a worse first run but not a leak.
var (
	reIn    = regexp.MustCompile(`(?is)\bIN\s*\(([^()]*)\)`)
	reAny   = regexp.MustCompile(`(?is)=\s*ANY\s*\(\s*ARRAY\s*\[([^\]]*)\]`)
	reLit   = regexp.MustCompile(`'((?:[^']|'')*)'`)
	reLenEq = regexp.MustCompile(`(?is)\b(?:char_length|length|octet_length)\s*\([^()]*\)\s*=\s*(\d+)`)
)

// checkValues returns the allowed values a CHECK spells out, or nil.
func checkValues(c Constraints) []string {
	for _, chk := range c.Checks {
		var body string
		if m := reAny.FindStringSubmatch(chk); m != nil {
			body = m[1]
		} else if m := reIn.FindStringSubmatch(chk); m != nil {
			body = m[1]
		} else {
			continue
		}
		var out []string
		for _, lit := range reLit.FindAllStringSubmatch(body, -1) {
			out = append(out, strings.ReplaceAll(lit[1], "''", "'"))
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// checkLength returns the exact length a CHECK requires, or 0.
func checkLength(c Constraints) int {
	for _, chk := range c.Checks {
		if m := reLenEq.FindStringSubmatch(chk); m != nil {
			n := 0
			for _, r := range m[1] {
				n = n*10 + int(r-'0')
				if n > maxFit*32 {
					return 0
				}
			}
			return n
		}
	}
	return 0
}

// labels is the closed set of values the column accepts, from its enum labels
// or from a CHECK, or nil when the column is open.
func labels(c Constraints) []string {
	if len(c.EnumLabels) > 0 {
		return c.EnumLabels
	}
	return checkValues(c)
}

// A closed column — an enum, or a CHECK with a value list — accepts nothing
// but its own labels, whatever category it was classified under
// (ARCHITECTURE.md section 5, "preserve what the application checks": a masked
// enum is a valid label). A street address in a column constrained to 'GB' and
// 'US' fails the load with 23514, and an arbitrary string in an enum column
// fails with 22P02, so every generator asks these two before it counts or
// builds anything of its own: labelDomain at the top of Domain, labelValue at
// the top of Mask.
//
// Three maskers do not call them because they already answer for a closed
// column: special_category (through generic and collapsed), null (NULL, or
// zeroValue, which is the first label) and fixed (its literal is the point).
func labelDomain(c Constraints) (int64, bool) {
	l := labels(c)
	if len(l) == 0 {
		return 0, false
	}
	return int64(len(l)), true
}

func labelValue(h [32]byte, c Constraints) (Value, bool) {
	l := labels(c)
	if len(l) == 0 {
		return Value{}, false
	}
	return Value{Text: l[newStream(h).intn(int64(len(l)))]}, true
}

// smallDomainCeiling is what "small" means for a column whose distinct sample
// count is unknown. ARCHITECTURE.md section 5 phrases the rule as a property of
// the domain — "at small d" — and its worked example is a diagnosis column with
// eight admissible values; a domain at or below this ceiling is a substitution
// frequency recovers whatever a sample would have said. There is no sample on
// the paths section 4 makes normal (a special category recognised by name
// alone, a partitioned table with no leaves), so the alternative to an absolute
// bound is silence, and silence is the direction that fails open.
const smallDomainCeiling = 64

// smallDomain is section 5's other domain rule for an admissible domain d:
// below twice the distinct sampled values the masking is a stable substitution
// over a small alphabet. With no samples it falls back to the ceiling above,
// and a closed column — an enum, a CHECK list — is small by construction,
// because substitution inside a fixed set of labels preserves the frequencies
// that recover it.
func smallDomain(d int64, c Constraints) bool {
	if c.Distinct > 0 {
		return d < 2*c.Distinct
	}
	if len(labels(c)) > 0 {
		return true
	}
	return d <= smallDomainCeiling
}

// ColumnDomain is the number of distinct values the *column* can hold, from
// its type, its length, its enum labels and the CHECK shapes this module
// parses. The admissible domain of a masked column is the smaller of this and
// the generator's own Domain (ARCHITECTURE.md section 5).
func ColumnDomain(c Constraints) int64 {
	if l := labels(c); len(l) > 0 {
		return int64(len(l))
	}
	switch c.TypeTag {
	case famBoolean:
		return 2
	case famInteger:
		return 1 << 32
	case famDate:
		// The PostgreSQL date range, 4713 BC to 5874897 AD, is far wider than
		// any date a person carries; the generator's own Domain is what bounds
		// a person_date column.
		return math.MaxInt64
	case famBytea, famBigint, famNumeric, famFloat, famUUID, famInet, famCIDR,
		famJSON, famJSONB, famHstore, famTimestamp:
		return math.MaxInt64
	case famMacaddr:
		return math.MaxInt64
	default:
		if n := room(c); n > 0 {
			// Printable ASCII is the honest floor for what a varchar(n) holds.
			return satPow(95, n)
		}
		return math.MaxInt64
	}
}

// epsilon is the collision probability the unique-index rule is written at:
// d_required = n² / 2ε at ε = 10⁻⁶ (ARCHITECTURE.md section 5).
const epsilonDenominator = 500_000 // 1 / (2 × 10⁻⁶)

// Required is d_required for a table of n rows: the number of distinct outputs
// a generator must have before a unique column can carry n rows at ε = 10⁻⁶.
// There is never a retry counter; a column the largest generator cannot carry
// is refused by name at plan.
func Required(rows int64) int64 {
	if rows <= 0 {
		return 0
	}
	return satMul(satMul(rows, rows), epsilonDenominator)
}

// MaxRows inverts Required: the largest --take or --cap a column with the
// given admissible domain can carry at ε = 10⁻⁶. The refusal prints it, so a
// person has a number to type rather than a rule to work out.
func MaxRows(domain int64) int64 {
	if domain <= 0 {
		return 0
	}
	return int64(math.Sqrt(float64(domain) / float64(epsilonDenominator)))
}
