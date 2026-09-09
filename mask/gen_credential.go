// SPDX-License-Identifier: Apache-2.0

package mask

// The second credential generator (T-0098).
//
// CatCredential shipped with one masker: the fixed literal $lazyslice$invalid,
// whose Domain() is 1 and says so. That is the right default — a password
// column full of well-formed bcrypt is a column somebody will eventually try to
// crack — but it is the wrong and only answer for a *unique* credential column.
// d_required is n²/2ε (ARCHITECTURE.md §5) and MaxRows(1) is zero, so a unique
// credential column was refused at exit 12 at every row count: "lower --take"
// was not an escape, and the only two left were --unmask, which copies the
// credential into the target verbatim, and mapping_file:. Torture testing put a
// number on it: of the thirty-seven --unmask flags the ten real schemas carry in
// internal/invariants/torture_catalogue_test.go, eighteen are tagged (T-0098)
// and are this one defect — six in supabase-auth alone (refresh_tokens.token,
// users.confirmation_token, recovery_token, email_change_token_new,
// email_change_token_current, reauthentication_token), eight in gitlab, two in
// mastodon, one each in calcom and discourse. An authentication schema is
// nothing but unique credentials.
//
// That eighteen counted the flags standing in the catalogue when this was
// written and was not a re-run measurement. It is one now (T-0112, re-checked by
// T-HARD-C's run): the eighteen flags are gone, the split is nineteen --unmask,
// seven --skip-table and one --key over twenty-seven flags, and that is what
// docs/TORTURE.md, ROADMAP.md's gate-5 line and the flags-by-kind assertion in
// internal/invariants/torture_test.go all carry.
//
// credential_unique does have end-to-end torture coverage now, and it is worth
// stating precisely rather than generously. Of the eighteen columns whose flag
// went, four hold masked values in a target — auth.refresh_tokens.token 136/136,
// auth.users.confirmation_token 100/100, auth.users.recovery_token 100/100 and
// public.user_security_keys.credential_id 100/100, every one carrying
// CredentialUniquePrefix with a distinct count equal to the row count. Two are
// the empty string mask.Apply passes through, ten are NULL in their fixture, and
// mastodon's two are in the run that refuses at exit 13 before the plan. The
// second end-to-end evidence is testdata/regressions/004 and 007, whose headers
// T-0113 re-cut from the exit-12 refusal this generator removed to `expect: ok`
// plus a `unique-masked:` assertion that reads the columns out of the target.
// docs/TORTURE.md's "Flags" section carries the same breakdown.
//
// So this is the alternate Pick escalates to, the same shape phone_unique and
// ip_unique already have for CatPhone and CatNetworkID: registered *after* the
// fixed literal, so it is never the default and a non-unique credential column
// still becomes $lazyslice$invalid.
//
// What it emits is still not a plausible credential. "lazyslice-invalid-" is
// the whole point of the value and it is the first eighteen characters of it:
// nothing that reaches this column can be mistaken for a token an application
// would accept, and a human reading the target sees where it came from. Only
// the suffix varies, and it is base32 of h like every other suffix in this
// module (gen_email.go's unique local part, gen_text.go's URL path), so the
// determinism contract is the module's own and nothing of the original
// survives.

// CredentialUniquePrefix opens every value credentialUniqueMasker emits. It is
// deliberately readable: the column's contents should announce the tool that
// wrote them, exactly as CredentialLiteral does.
const CredentialUniquePrefix = "lazyslice-invalid-"

// credentialUniqueSuffix is the number of base32 symbols the suffix carries
// when the column has room for all of them: 13 symbols are 65 bits, which
// clears the 2⁶⁴ ARCHITECTURE.md §5 asks of a generator a unique column
// escalates to.
const credentialUniqueSuffix = 13

// credentialUniqueSymbols is how many base32 symbols fit after the prefix, or 0
// when the column cannot hold the prefix and at least one symbol. Mask and
// Domain both read it, so the number Domain reports is the number of values
// Mask can actually produce.
//
// A column too narrow for the full thirteen is length-fitted rather than
// refused outright: the suffix shortens and Domain shrinks with it, which is
// the honest figure and lets Pick refuse on d_required rather than on room.
// Nothing is ever truncated — a value that did not fit is ErrNoRoom, not a
// prefix of one.
func credentialUniqueSymbols(c Constraints) int {
	n := credentialUniqueSuffix
	if c.MaxLen > 0 {
		budget := c.MaxLen - len(CredentialUniquePrefix)
		if budget < 1 {
			return 0
		}
		n = min(n, budget)
	}
	return n
}

// credentialUniqueMasker is the generator a credential column under a unique
// index gets: an unusable, self-identifying token whose suffix comes from h.
type credentialUniqueMasker struct{}

func (credentialUniqueMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	n := credentialUniqueSymbols(c)
	if n == 0 {
		return 0
	}
	return satPow(int64(len(b32alphabet)), n)
}

func (m credentialUniqueMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	n := credentialUniqueSymbols(c)
	if n == 0 {
		return Value{}, ErrNoRoom
	}
	out := CredentialUniquePrefix + newStream(h).base32(n)
	if !fits(out, c) {
		return Value{}, ErrNoRoom
	}
	// bytea takes the same bytes, the way the fixed literal does: the category
	// accepts bytea (writable.go) and a credential stored as bytes is still a
	// credential.
	if c.TypeTag == famBytea {
		return Value{Bytes: []byte(out)}, nil
	}
	return Value{Text: out}, nil
}
