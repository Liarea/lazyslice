//go:build tools

// This file is never built. It exists so that `go mod tidy` keeps the two
// dependencies the masking task needs, at the versions ARCHITECTURE.md section
// 13 pins: E.164 canonicalisation and validity for phones, and NFKC
// normalisation with case folding before hashing (section 5). Holding them
// with a blank import is cheaper than holding them with behaviour the scaffold
// has no test for.

package mask

import (
	_ "github.com/nyaruka/phonenumbers"
	_ "golang.org/x/text/cases"
	_ "golang.org/x/text/unicode/norm"
)
