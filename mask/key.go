// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// info is the HKDF info prefix. It carries the scheme version, so a change to
// the derivation is a new prefix and not a silent remapping.
const info = "lazyslice/v1/"

// NewKey returns 32 bytes from crypto/rand. A run that has no key file and no
// LAZYSLICE_SECRET uses one of these and discards it, which is why the fakes
// of two such runs do not match.
func NewKey() (Key, error) {
	var k Key
	if _, err := rand.Read(k[:]); err != nil {
		return Key{}, fmt.Errorf("mask: reading a run key: %w", err)
	}
	return k, nil
}

// ParseKey reads the text form of a key: 64 hex characters, with surrounding
// whitespace ignored. It is the form ./lazyslice.secret and $LAZYSLICE_SECRET
// carry. The error never quotes the input.
func ParseKey(s string) (Key, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return Key{}, fmt.Errorf("mask: %w", ErrKeyLength)
	}
	if len(b) != KeyLen {
		return Key{}, fmt.Errorf("mask: %w: got %d bytes", ErrKeyLength, len(b))
	}
	var k Key
	copy(k[:], b)
	return k, nil
}

// Fingerprint is sha256(K) as the first 8 hex characters, the value
// lazyslice.yml and lazyslice_meta carry as secret_fingerprint. It identifies
// a key without carrying it (THREAT_MODEL.md A4).
func (k Key) Fingerprint() string {
	sum := sha256.Sum256(k[:])
	return hex.EncodeToString(sum[:4])
}

// String is the fingerprint, never the key, so that a Key caught by a %v in a
// log line or an error string does not print secret material.
func (k Key) String() string { return "mask.Key(" + k.Fingerprint() + ")" }

// categoryKey is K_cat = HKDF-SHA256(K, salt=nil, info="lazyslice/v1/"+cat).
func (k Key) categoryKey(cat Category) ([]byte, error) {
	ck, err := hkdf.Key(sha256.New, k[:], nil, info+string(cat), KeyLen)
	if err != nil {
		return nil, fmt.Errorf("mask: deriving the %s key: %w", cat, err)
	}
	return ck, nil
}

// Digest is h = HMAC-SHA256(K_cat, Encode(typeTag, canonical)). It is the only
// source of variation a generator gets.
func Digest(k Key, cat Category, typeTag string, canonical []byte) ([32]byte, error) {
	ck, err := k.categoryKey(cat)
	if err != nil {
		return [32]byte{}, err
	}
	mac := hmac.New(sha256.New, ck)
	// hash.Hash never returns an error from Write.
	_, _ = mac.Write(Encode([]byte(typeTag), canonical))
	var h [32]byte
	copy(h[:], mac.Sum(nil))
	return h, nil
}

// Encode is the length-prefixed encoding of ARCHITECTURE.md section 5:
//
//	Encode(f1, ..., fn) = u32be(len(f1)) ‖ f1 ‖ ... ‖ u32be(len(fn)) ‖ fn
//
// It is unambiguous, which is the whole point: ("email", "a@b.com") and
// ("emai", "la@b.com") hash differently, and a column b.c in schema a never
// aliases column c in schema a.b. The residual filter in internal/transform
// keys on it too, so it is exported.
func Encode(fields ...[]byte) []byte {
	n := 0
	for _, f := range fields {
		n += 4 + len(f)
	}
	out := make([]byte, 0, n)
	var hdr [4]byte
	for _, f := range fields {
		// A PostgreSQL field is capped at 1 GiB, so a length cannot overflow
		// uint32 here.
		binary.BigEndian.PutUint32(hdr[:], uint32(len(f))) //nolint:gosec // G115: a column value cannot exceed 4 GiB
		out = append(out, hdr[:]...)
		out = append(out, f...)
	}
	return out
}
