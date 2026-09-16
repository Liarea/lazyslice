// SPDX-License-Identifier: Apache-2.0

// Package emit reads and writes lazyslice.yml (ARCHITECTURE.md section 10).
//
// The file is a record of what happened, not a configuration to be filled in:
// it is written on success, and re-running with it asks no questions. Read back
// it can only tighten (ADR-004) — a pattern may add a category or raise a
// confidence, an opt-out expires when the column's type changes, and nothing in
// it can lower a confidence or widen a slice.
//
// It never contains a secret and never contains a row value. A --where
// predicate holding a literal is withheld and recorded as where_fingerprint; a
// later run that finds a fingerprint and no --where stops with exit 2 asking for
// the predicate, rather than silently changing the slice.
//
// go-yaml is used rather than the standard library's because the emitted file
// carries the explanatory header comments that make it readable.
package emit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// Version is the yml's own schema version. A file carrying a higher one is
// refused rather than read: an unknown key is a safety setting this build does
// not implement, and reading it leniently is how an --unmask that never applied
// looks like one that did.
const Version = 1

// Options is what the run knows and ARCHITECTURE.md section 2's Emitter
// signature has no room for.
//
// Tool and SchemaFingerprint are two of section 10's top-level keys that no
// argument of Emit carries; Prior is the committed file this run read, which is
// what makes the merge possible at all; Unmask is the --unmask flag's own
// opt-outs, which arrive as a flag and must be recorded with `by: flag`.
//
// They are constructor arguments rather than new parameters on Emit for the
// reason internal/transform's schema and internal/verify's Options are:
// section 2 fixes the method's shape, and a stage that needs more than the
// shape carries it in the constructor.
type Options struct {
	// Tool is the version string the run was built as, written under `tool:`.
	Tool string
	// SchemaFingerprint is ADR-009's value, computed by internal/core through
	// load.SchemaFingerprint. Emit never computes one of its own: a second
	// definition of that hash binds no marker (ARCHITECTURE.md section 11.2).
	SchemaFingerprint string
	// Prior is the committed lazyslice.yml this run read, or nil on a first
	// run. The merge carries its opt-outs, its extra patterns and its mapping
	// files forward; nothing else in it survives, because everything else is a
	// record of a run that is over.
	Prior *pipeline.Config
	// Unmask is --unmask TABLE.COL=REASON, the reasons the flag gave. They are
	// recorded with `by: flag` so that the next run needs no flag.
	Unmask map[ref.ColumnRef]string
	// PasswordCommand is --password-command, recorded under `password_command`
	// as a command string and never its output (ARCHITECTURE.md section 8).
	PasswordCommand string
	// NotRecreated is Schema.NotRecreated as a count per object kind, section
	// 10's `not_recreated:` block. It comes in through the constructor because
	// Emit is given the plan and not the schema, and a count of the views a run
	// left behind is one of the things the file exists to state (section 11.1).
	NotRecreated map[string]int
}

type emitter struct{ opts Options }

// New returns the yml emitter.
func New(opts Options) pipeline.Emitter { return emitter{opts: opts} }

var _ pipeline.Emitter = emitter{}

// Emit builds the Config the writer records (ARCHITECTURE.md section 10).
//
// Every value it produces is an identifier, a count, a fingerprint or a flag
// value with literals withheld. It makes no decision: the classification, the
// plan and the request arrive already decided, and the only judgement here is
// which of them the file has to carry so that the next run asks nothing.
func (e emitter) Emit(
	plan *pipeline.Plan,
	cls *pipeline.Classification,
	report *pipeline.Report,
	req pipeline.PlanRequest,
	source, target pipeline.Candidate,
	keyFP string,
) (*pipeline.Config, error) {
	if plan == nil {
		return nil, errors.New("emit: no plan: the file records the slice that ran")
	}
	if cls == nil {
		return nil, errors.New("emit: no classification: a file with no columns re-classifies everything fresh")
	}

	cfg := &pipeline.Config{
		Version:           Version,
		Tool:              e.opts.Tool,
		Source:            source.Provenance,
		SourceRef:         source.Ref,
		SourceLabel:       source.Label,
		PasswordCommand:   e.opts.PasswordCommand,
		Target:            target.Provenance,
		TargetRef:         target.Ref,
		TargetLabel:       target.Label,
		Root:              plan.Root,
		Take:              plan.Take,
		Cap:               req.Cap,
		Caps:              req.TableCaps,
		Depth:             req.Depth,
		RowBudget:         req.RowBudget,
		MemoryBudget:      req.MemoryBudget,
		Keys:              req.Keys,
		Skipped:           plan.Skipped,
		SchemaFingerprint: e.opts.SchemaFingerprint,
		ClassFingerprint:  cls.Fingerprint,
		SnapshotID:        plan.SnapshotID,
		SecretFingerprint: keyFP,
		Columns:           map[ref.ColumnRef]pipeline.ColumnConfig{},
	}

	// The predicate, withheld when it carries a literal (THREAT_MODEL.md T5).
	if req.Where != "" {
		if HoldsLiteral(req.Where) {
			cfg.WhereFingerprint = WhereFingerprint(req.Where)
		} else {
			cfg.Where = req.Where
		}
	}

	if e.opts.Prior != nil {
		cfg.ExtraPatterns = e.opts.Prior.ExtraPatterns
	}

	for col, d := range cls.Decisions {
		cfg.Columns[col] = e.columnConfig(col, d)
	}

	cfg.Plan = e.planSummary(plan, cls, report)
	return cfg, nil
}

// columnConfig is one column's entry, with the merge applied.
//
// The record half is the decision: category, confidence, reason, the masker
// when the column was masked, and the type fingerprint the decision was made
// against. The merge half is what the committed file has to keep carrying: the
// opt-out that produced this decision. It is not derivable from a Decision, and
// losing it silently re-masks a column the operator opted out of on the next
// run.
//
// mapping_file is no longer part of the merge (ADR-012): pipeline.ColumnConfig
// carries no field for it, so there is nothing here to carry forward, and a
// prior file naming it is refused at Read before columnConfig ever runs.
func (e emitter) columnConfig(col ref.ColumnRef, d pipeline.Decision) pipeline.ColumnConfig {
	cc := pipeline.ColumnConfig{
		Category:   d.Category,
		Confidence: d.Confidence,
		Reason:     d.Reason,
		Unique:     d.UniqueIndex,
		TypeFP:     d.TypeFP,
	}
	if d.Masked {
		cc.Masker = d.Masker
	}

	switch d.Source {
	case pipeline.ByFlagUnmask:
		// The flag's own opt-out. It is written with this run's type
		// fingerprint, so the next run can expire it when the column changes —
		// which is the whole of THREAT_MODEL.md T3's control.
		if reason := e.opts.Unmask[col]; reason != "" {
			cc.Unmask = &pipeline.Unmask{Reason: reason, By: "flag", TypeFP: d.TypeFP}
		}
	case pipeline.ByYmlUnmask:
		// The file's own opt-out, honoured by classify, carried forward
		// verbatim. An expired one never reaches here: classify puts it in
		// Classification.Expired and the decision is not ByYmlUnmask, so the
		// merge drops it exactly when the classifier stopped honouring it.
		if e.opts.Prior != nil {
			if prior, ok := e.opts.Prior.Columns[col]; ok && prior.Unmask != nil {
				u := *prior.Unmask
				cc.Unmask = &u
			}
		}
	case pipeline.ByClassifier, pipeline.ByYmlRaise, pipeline.ByFKPropagation, pipeline.ByNeighbour:
	}
	return cc
}

// planSummary is the plan block: counts and identifiers only.
func (e emitter) planSummary(
	plan *pipeline.Plan, cls *pipeline.Classification, report *pipeline.Report,
) pipeline.PlanSummary {
	sum := pipeline.PlanSummary{
		SCCs:         plan.SCCs,
		Unreadable:   plan.Unreadable,
		VirtualFKs:   plan.Virtual,
		NotRecreated: e.opts.NotRecreated,
	}
	for _, s := range plan.Steps {
		switch s.Mode {
		case pipeline.Lookup:
			sum.Tables++
			sum.Lookups = append(sum.Lookups, s.Table)
		case pipeline.SchemaOnly:
			sum.Unreachable = append(sum.Unreachable, s.Table)
		case pipeline.ChildOK, pipeline.ParentOnly:
			sum.Tables++
		}
	}
	sum.Rows = plan.Estimate.Rows
	if report != nil {
		var loaded int64
		for _, n := range report.Rows {
			loaded += n
		}
		if loaded > 0 {
			sum.Rows = loaded
		}
	}
	for col, d := range cls.Decisions {
		if d.Masked && d.SmallDomain {
			sum.SmallDomain = append(sum.SmallDomain, col)
		}
	}
	sortColumns(sum.SmallDomain)
	if sum.NotRecreated == nil {
		sum.NotRecreated = map[string]int{}
	}
	return sum
}

// ---------- the file ----------

// header is the comment block section 10 opens with. go-yaml writes no comment
// of its own, so it is prepended to the encoded document; {tool} and {at} are
// the only substitutions, and neither is a value.
const header = `# lazyslice.yml — written by lazyslice {tool} on {at}. Commit it for CI.
# Re-running with this file asks no questions. Columns not listed here are
# classified fresh and masked at or above "possible"; opt-outs expire when the
# column's type changes. This file never contains a secret.
`

// Write writes the config to path.
//
// The file is written whole, through a temporary file in the same directory and
// a rename, so that an interrupted write leaves the committed file as it was
// rather than truncated: this file is what the next run reads to avoid asking
// questions, and half of it asks all of them.
func (emitter) Write(path string, cfg *pipeline.Config) error {
	if cfg == nil {
		return errors.New("emit: nothing to write")
	}
	// The one guard the writer owes THREAT_MODEL.md T5. Everything else in
	// Config is an identifier, a count or a fingerprint by construction; the
	// predicate is the single field that can hold a value the operator typed,
	// and Emit withholds it. A Config assembled some other way is refused here
	// rather than written.
	if cfg.Where != "" && HoldsLiteral(cfg.Where) {
		return fmt.Errorf("emit: refusing to write %s: the --where predicate holds a literal "+
			"and was not withheld (ARCHITECTURE.md section 10, THREAT_MODEL.md T5)", path)
	}

	body, err := yaml.Marshal(toDocument(cfg))
	if err != nil {
		return fmt.Errorf("emit: encoding %s: %w", path, err)
	}
	text := strings.NewReplacer(
		"{tool}", orUnknown(cfg.Tool),
		"{at}", time.Now().UTC().Format(time.RFC3339),
	).Replace(header) + string(body)

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".lazyslice-*.yml")
	if err != nil {
		return fmt.Errorf("emit: creating a temporary file beside %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("emit: writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("emit: writing %s: %w", path, err)
	}
	// 0o644: the file is committed and read by everyone on the team, and it
	// holds no secret by construction.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("emit: setting the mode of %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("emit: replacing %s: %w", path, err)
	}
	return nil
}

// Read parses a lazyslice.yml. A file that is not there is os.ErrNotExist,
// which the caller reads as "first run" rather than as a failure.
func (emitter) Read(path string) (*pipeline.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc document
	if parseErr := unmarshal(b, &doc); parseErr != nil {
		return nil, fmt.Errorf("emit: reading %s: %w", path, parseErr)
	}
	if doc.Version > Version {
		return nil, fmt.Errorf("emit: %s is version %d and this lazyslice reads version %d: "+
			"upgrade lazyslice, or delete the file and let this run write a new one",
			path, doc.Version, Version)
	}
	cfg, err := doc.config()
	if err != nil {
		return nil, fmt.Errorf("emit: reading %s: %w", path, err)
	}
	return cfg, nil
}

// ---------- the predicate ----------

// HoldsLiteral reports whether a --where predicate carries a string or numeric
// literal, which section 10 withholds from the file.
//
// It is deliberately generous: a quote of any kind, a digit anywhere, or a
// dollar sign (a dollar-quoted string, or a parameter) is a literal. A
// predicate the tool wrongly withholds costs the operator the flag on the next
// run and says so; one it wrongly records puts a production value in a
// committed file, and there is no second chance at that (THREAT_MODEL.md T5).
func HoldsLiteral(where string) bool {
	return strings.ContainsAny(where, "'\"$0123456789")
}

// WhereFingerprint is sha256(where)[:16], what a withheld predicate is recorded
// as. It identifies the predicate without carrying it, so a later run can say
// "this file was written with a --where you have not passed".
func WhereFingerprint(where string) string {
	sum := sha256.Sum256([]byte(where))
	return hex.EncodeToString(sum[:8])
}

// ---------- the password command ----------

// PasswordCommandSuspicious is the 2026-09-16 round 2 red team's R2-16 check
// (THREAT_MODEL.md T5, T6): a --password-command string is screened before
// Emit writes it into lazyslice.yml, whose header promises the file "never
// contains a secret". `--password-command 'echo Hunter2InCommand'` recorded
// the command string verbatim, and the string *is* the password.
//
// The heuristic splits the command the way a shell would — quotes honoured,
// nothing executed — and requires every word to be unambiguously *not* a
// literal, rather than trusting anything that merely contains a slash: an
// argument is accepted only when it is path-shaped (anchored with `/`, `./`
// or `../`, each segment drawn only from letters, digits, `.`, `_` and `-`)
// or names a file that actually exists. An *unanchored* multi-segment word
// (`db/prod/password`) is not, by itself, path-shaped — a slash-bearing
// password such as `kX9mQ2/vT4ns8Lb1` takes exactly that shape and would
// otherwise be recorded verbatim — so an unanchored argument is trusted only
// when it names a file that exists, via the same exists-on-disk test as
// every other non-path-shaped word; a credential helper invoked with an
// unanchored key that happens not to exist on this machine is screened, the
// conservative direction. A base64 password such as `aB3/xY9+QzT=` carries a
// `/` about a third of the time at 24 characters but also carries `+` and
// `=`, which are not in that character class, so it fails the shape test and
// is screened; a Windows-style value like `C:\Users\bob` has no `/` at all
// and fails it too — a real `\`-separated path is instead caught by the
// exists-on-disk half, which stats the argument as the host OS would resolve
// it. The one-word case (no arguments at all) gets a third way to be safe:
// naming a program findable on PATH, since `--password-command
// Hunter2InCommand` — a bare word, no path shape, resolving to nothing on
// PATH — is a plausible misreading of what the flag takes and not a command,
// so it is screened rather than recorded as though a program name explained
// it. A multi-word command's own first word (the program) keeps the older,
// looser rule and is always accepted, because a credential-helper program is
// routinely invoked by a bare, unqualified name (`vault-fetch db/prod/pass`)
// and PATH lookup at record time would depend on the machine running lazyslice
// rather than the shell that will actually run the command.
//
// It is deliberately narrow rather than clever: a flag with a literal value
// baked in (`-p supersecret`) and a bare credential-helper subcommand
// (`vault read`, without an argument that looks like a path) both fail the
// shape test and are screened too, which is the conservative direction — a
// legitimate command this trips is withheld and printed as withheld, on a
// --password-command that can be typed again next run; a password this let
// through would be in a committed file with no second chance (the same
// argument HoldsLiteral makes for --where).
//
// A command that cannot be parsed as shell words at all (an unterminated
// quote), or that parses to no words at all (blank or whitespace-only), is
// also suspicious: neither is a command lazyslice can run, and nothing about
// either makes it safe to record verbatim.
func PasswordCommandSuspicious(cmd string) bool {
	words, err := splitCommandWords(cmd)
	if err != nil || len(words) == 0 {
		return true
	}
	if len(words) == 1 {
		return programNameSuspicious(words[0])
	}
	for _, w := range words[1:] {
		if argumentSuspicious(w) {
			return true
		}
	}
	return false
}

// pathSegmentRE is the character class a path component may hold for
// PasswordCommandSuspicious to trust it: no `+`, `=`, spaces or other
// punctuation a password generator reaches for, and nothing a shell would
// need quoting to pass through literally.
var pathSegmentRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// pathShaped reports whether w is anchored like a path (`/`, `./`, `../`).
// An unanchored word — whether a bare single segment (`Hunter2InCommand`) or
// a bare run of `/`-joined segments (`db/prod/password`) — is never
// path-shaped on its own: a slash by itself does not distinguish a relative
// path from a slash-bearing literal (a base64 password carries one about a
// third of the time), so pathShaped trusts only the anchored forms and
// leaves every unanchored word, single- or multi-segment, to
// argumentSuspicious's exists-on-disk test (and, for the one-word case,
// programNameSuspicious's on-PATH test). This is the R2-16 gap this function
// exists to close.
func pathShaped(w string) bool {
	var body string
	switch {
	case strings.HasPrefix(w, "../"):
		body = w[3:]
	case strings.HasPrefix(w, "./"):
		body = w[2:]
	case strings.HasPrefix(w, "/"):
		body = w[1:]
	default:
		return false
	}
	if body == "" {
		return false
	}
	for _, seg := range strings.Split(body, "/") {
		if seg == "" || !pathSegmentRE.MatchString(seg) {
			return false
		}
	}
	return true
}

// argumentSuspicious reports whether w, a word after the program name, is not
// unambiguously safe to record: it is trusted only when it is path-shaped or
// names a file that exists, exactly as the doc comment on
// PasswordCommandSuspicious describes.
func argumentSuspicious(w string) bool {
	if pathShaped(w) {
		return false
	}
	_, err := os.Stat(w)
	return err != nil
}

// programNameSuspicious applies argumentSuspicious's test to a one-word
// command (there is no argument to have leaked a password into, only the
// question of whether this is a command at all) and adds a third way to be
// safe: resolving to a program on PATH, which a shape or existence check
// alone cannot tell from a bare literal.
func programNameSuspicious(w string) bool {
	if !argumentSuspicious(w) {
		return false
	}
	_, err := exec.LookPath(w)
	return err != nil
}

// splitCommandWords splits cmd into shell-like words: whitespace separates
// words outside quotes, and single or double quotes group one word without
// being part of it. It does no variable expansion and executes nothing — it
// exists only to tell a program name and a path-shaped argument from a bare
// literal, never to run the command.
func splitCommandWords(cmd string) ([]string, error) {
	var (
		words  []string
		cur    strings.Builder
		inWord bool
		quote  rune
	)
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for _, r := range cmd {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			inWord = true
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("emit: %q is not a closed shell word", cmd)
	}
	flush()
	return words, nil
}

// ---------- sizes ----------

// FormatSize renders a byte count the way --memory-budget spells it.
func FormatSize(n int64) string {
	switch {
	case n == 0:
		return "0"
	case n%(1<<30) == 0:
		return strconv.FormatInt(n>>30, 10) + "GiB"
	case n%(1<<20) == 0:
		return strconv.FormatInt(n>>20, 10) + "MiB"
	case n%(1<<10) == 0:
		return strconv.FormatInt(n>>10, 10) + "KiB"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// ParseSize reads the --memory-budget spelling: a count of bytes, or one
// followed by KiB, MiB, GiB (or K, M, G, KB, MB, GB, which mean the same thing
// here — a budget is not the place to argue about 1000 against 1024).
func ParseSize(s string) (int64, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, errors.New("emit: no size")
	}
	unit := int64(1)
	for _, suffix := range []struct {
		text  string
		scale int64
	}{
		{"KiB", 1 << 10}, {"MiB", 1 << 20}, {"GiB", 1 << 30},
		{"KB", 1 << 10}, {"MB", 1 << 20}, {"GB", 1 << 30},
		{"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30},
	} {
		if len(t) > len(suffix.text) && strings.EqualFold(t[len(t)-len(suffix.text):], suffix.text) {
			unit = suffix.scale
			t = strings.TrimSpace(t[:len(t)-len(suffix.text)])
			break
		}
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("emit: %q is not a size: pass a byte count, or one with KiB, MiB or GiB", s)
	}
	return n * unit, nil
}

// ---------- helpers ----------

// unmarshal is the single decoding entry point, kept here so that every read of
// the yml goes through the same options. Strict mode is deliberate: an unknown
// key in lazyslice.yml is a typo in a safety setting, and silently ignoring it
// is how an --unmask that never applied looks like one that did.
func unmarshal(b []byte, v any) error {
	return yaml.UnmarshalWithOptions(b, v, yaml.Strict())
}

// Decode parses a lazyslice.yml body into any shape. It is exported for a
// caller that wants the raw document rather than a Config.
func Decode(b []byte, v any) error { return unmarshal(b, v) }

func orUnknown(s string) string {
	if s == "" {
		return "dev"
	}
	return s
}

func sortColumns(cs []ref.ColumnRef) {
	sort.Slice(cs, func(i, j int) bool { return cs[i].Less(cs[j]) })
}

// maskerID keeps the yaml decoder honest about mask.ID, which is a string type.
func maskerID(s string) mask.ID { return mask.ID(s) }

// refOf is dsn.Ref from the fields the file carries.
//
// params goes through dsn.FilterAllowedParams rather than straight onto the
// Ref: the committed file is text a person can hand-edit, and a `params:`
// block is not exempt from "Ref never holds a credential" just because it
// came from a read instead of a fresh Parse (THREAT_MODEL.md T4, T5).
func refOf(host string, port int, database, user string, params map[string]string) dsn.Ref {
	return dsn.Ref{Host: host, Port: port, Database: database, User: user, Params: dsn.FilterAllowedParams(params)}
}
