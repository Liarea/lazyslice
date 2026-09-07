// SPDX-License-Identifier: Apache-2.0

// Command lazyslice snapshots a production SQL database into a safe local copy:
// subset by a root table, follow foreign keys, mask personal data, load.
//
// This file is the whole CLI surface for v1 (ARCHITECTURE.md section 8). It
// builds a core.Request from flags, picks a renderer, and calls core.Run. It
// contains no pipeline logic and reaches no stage directly, which is what lets
// internal/tui build the same Request from keystrokes without duplicating
// anything: the TUI is a thin layer, and every action it offers is a flag here
// first (ADR-002).
//
// A run connects to both databases, drops and recreates the target's schema,
// loads the masked slice, and writes ./lazyslice.yml and (on a first run)
// ./lazyslice.secret. `introspect`, `classify`, `plan` and `doctor` stop before
// the target is touched; `verify` does not — it re-runs the whole slice (see
// its Short text).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/emit"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/render"
)

// Exit codes (ADR-005). They are part of the interface: CI jobs branch on them,
// so a code never changes meaning.
const (
	ExitOK                 = 0
	ExitInternal           = 1 // scaffold and unexpected failures; not an ADR-005 code
	ExitUsage              = 2 // bad flags, or a refusal the user can fix by rewording
	ExitNoSource           = 3 // nothing to read from
	ExitTargetRefused      = 4 // the gate said no
	ExitCredential         = 5 // no password, or --require-key with no key
	ExitWritableRole       = 6 // --require-read-only-role and the role can write
	ExitExtractOrLoad      = 7
	ExitForeignKey         = 8
	ExitResidual           = 9  // a masked value reached the target, or a hit could not be confirmed
	ExitDrift              = 10 // --strict-schema and a column the yml has never seen
	ExitBudget             = 11 // --row-budget or --memory-budget exceeded
	ExitPlanRefused        = 12 // no row identity, an unreadable parent, a domain too small to mask
	ExitSchemaNotRecreated = 13 // a recreated object depends on one v1 does not recreate
	ExitInterrupted        = 130
)

// Build information, set by goreleaser through -ldflags. The zero values are
// what a `go build` without them produces, and `lazyslice version` says so
// rather than printing an empty line.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// supportedMajors is the range of Postgres server versions v1 supports
// (ADR-003). It is printed by `version` so that a bug report carries it.
const supportedMajors = "14 to 18"

func main() { os.Exit(main1()) }

// main1 exists so that the signal handler is torn down before os.Exit, which
// never runs a deferred call.
func main1() int {
	// SIGINT and SIGTERM cancel the context; core.Run rolls the target
	// transaction back and exits 130 rather than leaving a half-written target.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, os.Args[1:], os.Stdout, os.Stderr)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	// The build's own version is what the yml records as `tool:` and what the
	// marker table records as tool_version (ARCHITECTURE.md section 11.2). It
	// is set here, once, because -ldflags reaches main and nothing else.
	core.Version = version

	req := core.NewRequest()
	root := newCommandTree(ctx, &req, stdout)

	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.ExecuteContext(ctx); err != nil {
		// The request is read, not the flag variable, because
		// --show-row-values-in-errors is the one thing report needs to know and
		// the request is where every flag lands.
		return report(stderr, err, req.ShowRowValuesInErrors)
	}
	return ExitOK
}

// newCommandTree assembles the whole CLI: the root command, the five stage
// subcommands, version, and the flag groups. It is one function so that the
// test that locks the flag surface against ARCHITECTURE.md section 8 sees
// exactly what a user sees.
func newCommandTree(ctx context.Context, req *core.Request, stdout io.Writer) *cobra.Command {
	flags := newFlagGroups()
	raw := newRawFlags()

	bindFlags(flags, req, raw)

	root := newRootCmd(ctx, req, raw, stdout)
	for _, g := range flags {
		root.PersistentFlags().AddFlagSet(g.set)
	}
	root.SetUsageFunc(groupedUsage(flags))
	// Cobra reports a mistyped flag, an unparseable value and too many
	// positional arguments as plain errors, which report would map to
	// ExitInternal. They are the operator's most common mistake and they are
	// usage errors, so they are wrapped here into errUsage and exit 2 like
	// every hand-written one (ADR-005). FlagErrorFunc walks to the parent, so
	// setting it on the root covers the subcommands too.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %w", errUsage, err)
	})
	// The subcommand list is exactly the one in ARCHITECTURE.md section 8;
	// cobra's generated completion command is not on it.
	root.CompletionOptions.DisableDefaultCmd = true
	root.SilenceUsage = true
	root.SilenceErrors = true

	root.AddCommand(
		subcommand(ctx, "introspect", core.ModeIntrospect,
			"Read the source catalog and print it", req, raw, stdout),
		subcommand(ctx, "classify", core.ModeClassify,
			"Classify every column and print the reasons", req, raw, stdout),
		subcommand(ctx, "plan", core.ModePlan,
			"Print the subset plan and stop", req, raw, stdout),
		// The Short text says "re-run the slice" and not "re-run the checks",
		// because that is what it does: §6 item 1's residual filter is random
		// per run and never leaves the process, so it cannot be rebuilt from a
		// target, and a verify that skipped the residual scan would print a
		// green tick over the one check the tool exists for. Until §6 says how a
		// standalone verify reconstructs the filter, ModeVerify runs the whole
		// pipeline — which truncates the target and takes a fresh snapshot hold
		// on the source — and the help text has to say so (internal/core).
		subcommand(ctx, "verify", core.ModeVerify,
			"Re-run the whole slice into the target and re-check it (drops and reloads the target)",
			req, raw, stdout),
		subcommand(ctx, "doctor", core.ModeDoctor,
			"Print what lazyslice can see and what it cannot prove", req, raw, stdout),
		newVersionCmd(stdout),
	)

	return root
}

// report maps an error to an exit code and prints it once. Every mapping here
// is an ADR-005 code; anything unmapped is ExitInternal, because a code that
// means "something went wrong" must not be confused with one a CI job branches
// on.
func report(stderr io.Writer, err error, showValues bool) int {
	var stop *core.Stop
	switch {
	case errors.As(err, &stop):
		// Every stage refusal arrives as one of these, carrying the event code
		// and the ADR-005 exit that go with it (internal/core). The line the
		// user reads was already rendered from the catalogue by the sink; this
		// is the developer-facing half and the exit code.
		//
		// It is tested *before* context.Canceled on purpose. A stage refusal
		// often cancels something on its way out — a masker that refuses a value
		// cancels extract — so a Stop can carry a cancellation underneath it,
		// and errors.Is would reach it through Unwrap. Matching that first would
		// print "interrupted" and return 130 for a masking refusal, while the
		// Error event the sink already printed carried a different code and a
		// different exit. core.asStop maps a genuine cancellation to
		// CodeInterrupted with exit 130, so this branch answers that case too
		// and the two can no longer disagree.
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		return stop.Exit
	case errors.Is(err, context.Canceled):
		// A cancellation that never reached core: cobra's own context, or a
		// signal during flag parsing.
		fmt.Fprintln(stderr, "lazyslice: interrupted")
		return ExitInterrupted
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		return ExitUsage
	case errors.Is(err, pipeline.ErrNotImplemented):
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		fmt.Fprintln(stderr, "  that stage is not in this build")
		return ExitInternal
	default:
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		return ExitInternal
	}
}

// renderSafe is the only thing report is allowed to print, so that the single
// error egress of the binary has one redaction pass rather than none.
//
// A *pgconn.PgError quotes the conflicting row in Detail and Where, so it is
// rendered by internal/pg, which drops those fields unless
// --show-row-values-in-errors (THREAT_MODEL.md T4). An error carrying a
// connection string must be redacted where it is created, by the package that
// holds the dsn.DSN: report never sees the credential and so cannot know what
// to remove.
func renderSafe(err error, showValues bool) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pg.RenderError(pgErr, showValues)
	}
	return err.Error()
}

// oneDSN is cobra.MaximumNArgs(1) with its error wrapped, so that "too many
// arguments" is exit 2 like every other usage error rather than exit 1.
func oneDSN(cmd *cobra.Command, args []string) error {
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	return nil
}

// errUsage marks a flag the user must fix. It is wrapped, never returned bare,
// so the message always says which flag and what shape it wanted.
var errUsage = errors.New("usage")

// ---------- commands ----------

func newRootCmd(ctx context.Context, req *core.Request, raw *rawFlags, stdout io.Writer) *cobra.Command {
	var showVersion bool

	cmd := &cobra.Command{
		Use:   "lazyslice [DSN] [flags]",
		Short: "Snapshot a production database into a safe local copy",
		Long: "lazyslice subsets a production SQL database by a root table, follows its\n" +
			"foreign keys, masks personal data, and loads the result into a local\n" +
			"database. It never writes to the source, and it refuses to write to a\n" +
			"target that is not empty or was not written by lazyslice.",
		Args:         oneDSN,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprint(stdout, versionText())
				return nil
			}
			if len(args) == 1 {
				req.Source = args[0]
			}
			if err := finish(cmd, req, raw); err != nil {
				return err
			}
			_, err := core.Run(ctx, *req, sinkFor(*req, stdout))
			return err
		},
	}

	cmd.Flags().BoolVar(&showVersion, "version", false,
		"Print the version, commit, Go version and supported Postgres majors")

	return cmd
}

// subcommand builds one of the five stage subcommands. Each stops the pipeline
// at its own stage and prints what it found; each accepts --json.
func subcommand(
	ctx context.Context,
	name string,
	mode core.Mode,
	short string,
	req *core.Request,
	raw *rawFlags,
	stdout io.Writer,
) *cobra.Command {
	return &cobra.Command{
		Use:          name + " [DSN] [flags]",
		Short:        short,
		Args:         oneDSN,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				req.Source = args[0]
			}
			// The mode is what tells the five subcommands apart inside
			// core.Run; without it introspect, classify, verify and doctor
			// build the same Request (T-0020's log, closed by T-CORE).
			req.Mode = mode
			if mode == core.ModePlan {
				req.PlanOnly = true
			}
			if err := finish(cmd, req, raw); err != nil {
				return err
			}
			if mode == core.ModeIntrospect {
				return introspect(ctx, *req, stdout)
			}
			_, err := core.Run(ctx, *req, sinkFor(*req, stdout))
			return err
		},
	}
}

// introspect runs `lazyslice introspect`. With --json it prints the
// pipeline.SchemaSummary of ARCHITECTURE.md section 2 on stdout; without it the
// event stream has already said what was found.
//
// The summary's fingerprint is ADR-009's, computed by internal/core through
// load.SchemaFingerprint like every other use of that hash. There is no second
// definition anywhere, and an empty one is printed as an empty key rather than
// as a value computed here.
//
// It is written unindented, on one line, because --json is "NDJSON events on
// stdout" (ARCHITECTURE.md section 8) and the NDJSON sink is writing to the
// same stream: an indented object contributes fifteen lines that are not JSON
// on their own, and every consumer doing line-by-line json.Unmarshal breaks on
// the first of them. TestIntrospectJSONIsOneObjectPerLine holds it.
func introspect(ctx context.Context, req core.Request, stdout io.Writer) error {
	sink := sinkFor(req, stdout)
	summary, err := core.Introspect(ctx, req, sink)
	if err != nil {
		return err
	}
	if !req.JSON {
		return nil
	}
	return writeSummary(stdout, summary)
}

// writeSummary writes the schema summary as one JSON object on one line.
//
// It is a function of its own so that the rule can be tested without a
// database: --json means NDJSON, the event sink is writing to the same stream,
// and an indented object would put fifteen lines that are not JSON into it.
func writeSummary(w io.Writer, summary *pipeline.SchemaSummary) error {
	return json.NewEncoder(w).Encode(summary)
}

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit, Go version and supported Postgres majors",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprint(stdout, versionText())
			return nil
		},
	}
}

func versionText() string {
	rev := commit
	if rev == "none" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" {
					rev = s.Value
				}
			}
		}
	}
	return fmt.Sprintf(
		"lazyslice %s\n  commit    %s\n  built     %s\n  go        %s\n  postgres  %s\n",
		version, rev, date, runtime.Version(), supportedMajors,
	)
}

func sinkFor(req core.Request, stdout io.Writer) event.Sink {
	if req.JSON {
		return render.NewNDJSON(stdout)
	}
	return render.NewLines(stdout)
}

// ---------- flags ----------

// rawFlags holds the flag values whose shape needs parsing before they reach
// core.Request: the repeatable TABLE=VALUE ones. Parsing them in one place, at
// the end, is what lets a bad shape be a single clear usage error rather than
// five different ones.
type rawFlags struct {
	caps       []string // --cap, either "N" or "TABLE=N"
	keys       []string // --key TABLE=COL,COL
	unmask     []string // --unmask TABLE.COL=REASON
	skipTables []string // --skip-table TABLE
}

func newRawFlags() *rawFlags { return &rawFlags{} }

// flagGroup is one --help section. --help is grouped by stage, because the flag
// that changes a number is printed beside that number in the plan and a
// developer reads them in that order.
type flagGroup struct {
	title string
	set   *pflag.FlagSet
}

func newFlagGroups() []flagGroup {
	names := []string{"discover", "plan", "classify", "transform", "extract", "load and verify", "config", "render"}
	groups := make([]flagGroup, 0, len(names))
	for _, n := range names {
		groups = append(groups, flagGroup{title: n, set: pflag.NewFlagSet(n, pflag.ContinueOnError)})
	}
	return groups
}

func bindFlags(groups []flagGroup, req *core.Request, raw *rawFlags) {
	byTitle := map[string]*pflag.FlagSet{}
	for _, g := range groups {
		byTitle[g.title] = g.set
	}

	discover := byTitle["discover"]
	discover.StringVar(&req.Source, "source", "",
		"Names the source; a non-Postgres scheme or unsupported major exits 2")
	discover.StringVar(&req.Target, "target", "",
		"Names the target; never bypasses the gate")
	discover.StringVar(&req.DockerHost, "docker-host", "",
		"Docker endpoint for container discovery (default: DOCKER_HOST, context, default sockets)")
	discover.StringVar(&req.PasswordCommand, "password-command", "",
		"Command whose stdout is the password; recorded in the yml as a reference, never its output")
	discover.BoolVar(&req.CreateTarget, "create-target", false,
		"Start postgres:<source major> as lazyslice-target-<project> instead of asking")
	discover.StringVar(&req.AllowRemoteTarget, "allow-remote-target", "",
		"Permit a non-local target whose host equals HOST; without it a remote target is exit 4")
	discover.BoolVar(&req.RequireReadOnlyRole, "require-read-only-role", false,
		"Exit 6 when the source role holds INSERT, UPDATE or DELETE")
	discover.BoolVar(&req.Reconfigure, "reconfigure", false,
		"Ignore an existing lazyslice.yml and run the first-run path")

	plan := byTitle["plan"]
	plan.StringVar(&req.Root, "root", "",
		"Root table (default: computed from the foreign-key graph)")
	plan.IntVarP(&req.Take, "take", "n", core.DefaultTake,
		"Root rows, ORDER BY identity LIMIT N")
	plan.StringVar(&req.Where, "where", "",
		"Root predicate instead of LIMIT ordering; a literal is withheld from the yml")
	plan.StringArrayVar(&raw.caps, "cap", nil,
		"Children per parent key per edge, as N or TABLE=N (default 100); repeatable")
	plan.IntVar(&req.Depth, "depth", core.DefaultDepth,
		"Child depth from the root")
	plan.Int64Var(&req.RowBudget, "row-budget", core.DefaultRowBudget,
		"Abort planning above this many rows (exit 11)")
	plan.StringVar(&req.MemoryBudget, "memory-budget", core.DefaultMemoryBudget,
		"Abort planning above this estimated resident set (exit 11)")
	plan.StringArrayVar(&raw.keys, "key", nil,
		"Row identity for a table with no key, as TABLE=COL,COL; repeatable")
	plan.StringArrayVar(&raw.skipTables, "skip-table", nil,
		"Drop a child-only table to schema-only; repeatable")
	plan.BoolVar(&req.PlanOnly, "plan", false,
		"Stop after printing the plan; touch nothing")

	classify := byTitle["classify"]
	classify.StringArrayVar(&raw.unmask, "unmask", nil,
		"Per-column opt-out, as TABLE.COL=REASON; the bare form is exit 2; repeatable")
	classify.BoolVar(&req.StrictSchema, "strict-schema", false,
		"Exit 10 on any column the committed yml has never seen")

	transform := byTitle["transform"]
	transform.StringVar(&req.SecretFile, "secret-file", core.DefaultSecretFile,
		"Masking key file; LAZYSLICE_SECRET overrides it")
	transform.BoolVar(&req.RequireKey, "require-key", false,
		"Exit 5 instead of using an ephemeral key")

	extract := byTitle["extract"]
	extract.BoolVar(&req.SingleConnection, "single-connection", false,
		"Serialised extract on one connection (automatic behind a pooler)")

	loadVerify := byTitle["load and verify"]
	loadVerify.IntVar(&req.ResidualProbeCap, "residual-probe-cap", core.DefaultResidualProbeCap,
		"Source confirmation probes per run; hits beyond it are unconfirmable and exit 9")
	loadVerify.BoolVar(&req.ShowRowValuesInErrors, "show-row-values-in-errors", false,
		"Keep Detail and Where in rendered Postgres errors")

	cfg := byTitle["config"]
	cfg.StringVar(&req.ConfigPath, "config", core.DefaultConfigPath,
		"Config to read on re-run and write on success")
	cfg.BoolVar(&req.NoConfig, "no-config", false,
		"Do not write lazyslice.yml")
	cfg.BoolVar(&req.Yes, "yes", false,
		"Headless: ask nothing; questions with no safe default become hard failures naming their flag")

	rend := byTitle["render"]
	rend.BoolVar(&req.JSON, "json", false,
		"NDJSON events on stdout")
	rend.BoolVar(&req.TUI, "tui", false,
		"Enter the reasons and plan screens")
	rend.BoolVar(&req.Debug, "debug", false,
		"Stack traces and the statement trace on error")
}

// finish parses the TABLE=VALUE flags into the request, records which flags the
// operator actually passed, and refuses the values that cannot mean what they
// say.
//
// cmd may be nil, which is how the flag-shape tests drive it; the only thing it
// is used for is Changed, and a request with no Explicit set behaves as though
// every value came from a default.
func finish(cmd *cobra.Command, req *core.Request, raw *rawFlags) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: cannot read the working directory: %w", errUsage, err)
	}
	req.Workdir = wd

	// Which flags were typed. lazyslice.yml supplies a default for a value the
	// operator did not pass and never overrides one they did (ARCHITECTURE.md
	// section 10), and an int flag bound to its own default is otherwise
	// indistinguishable from one that was typed.
	if req.Explicit == nil {
		req.Explicit = map[string]bool{}
	}
	if cmd != nil {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				req.Explicit[f.Name] = true
			}
		})
	}
	// --cap is two flags wearing one name: a bare N that sets the global cap and
	// a TABLE=N that sets one table's. pflag knows only that the flag was
	// changed, so `--cap public.payment=10` would record Explicit["cap"] and
	// internal/core would then ignore the committed yml's global `cap:` —
	// silently reverting every other table to the built-in default. The
	// VisitAll answer is dropped here and parseCaps sets the key only for the
	// bare form, which is the only form that means "the global cap".
	delete(req.Explicit, "cap")

	if err := parseCaps(req, raw); err != nil {
		return err
	}
	if err := parseKeys(req, raw); err != nil {
		return err
	}
	if err := parseUnmask(req, raw); err != nil {
		return err
	}
	if err := checkCounts(req); err != nil {
		return err
	}

	for _, t := range raw.skipTables {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("%w: --skip-table wants a table name", errUsage)
		}
		req.SkipTables = append(req.SkipTables, t)
	}
	return nil
}

// checkCounts refuses the counts that have no meaning.
//
// A zero take, cap or depth is a usage error rather than a silent default,
// because pipeline.PlanRequest cannot carry the difference: an int field where
// zero means both "unset" and "none" would make `--take 0` slice 500 rows, and
// a flag that does the opposite of what it says is worse than one that refuses
// (T-CORE, 2026-09-06). A negative one is refused for the same reason.
func checkCounts(req *core.Request) error {
	for _, c := range []struct {
		flag  string
		value int64
		what  string
	}{
		{"take", int64(req.Take), "root rows"},
		{"cap", int64(req.Cap), "children per parent key"},
		{"depth", int64(req.Depth), "child depth"},
		{"row-budget", req.RowBudget, "rows"},
	} {
		if req.Explicit[c.flag] && c.value < 1 {
			return fmt.Errorf("%w: --%s wants a count of 1 or more (%s), got %d",
				errUsage, c.flag, c.what, c.value)
		}
	}
	if _, err := emit.ParseSize(req.MemoryBudget); err != nil {
		return fmt.Errorf("%w: --memory-budget wants a size such as 256MiB, got %q",
			errUsage, req.MemoryBudget)
	}
	return nil
}

// parseCaps reads --cap, which is either a bare N or TABLE=N, repeatable.
//
// An empty table name and a cap below 1 are both refused: `--cap =10` and
// `--cap public.payment=0` would each be stored and then match nothing, which is
// a flag that appears to have worked.
func parseCaps(req *core.Request, raw *rawFlags) error {
	const shape = "--cap wants N or TABLE=N, with N at least 1"
	for _, c := range raw.caps {
		table, value, found := strings.Cut(c, "=")
		if !found {
			n, err := strconv.Atoi(strings.TrimSpace(c))
			if err != nil || n < 1 {
				return fmt.Errorf("%w: %s, got %q", errUsage, shape, c)
			}
			req.Cap = n
			req.Explicit["cap"] = true
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 1 || strings.TrimSpace(table) == "" {
			return fmt.Errorf("%w: %s, got %q", errUsage, shape, c)
		}
		req.TableCaps[table] = n
	}
	return nil
}

// parseKeys reads --key TABLE=COL,COL.
func parseKeys(req *core.Request, raw *rawFlags) error {
	for _, k := range raw.keys {
		table, cols, found := strings.Cut(k, "=")
		if !found || strings.TrimSpace(table) == "" || strings.TrimSpace(cols) == "" {
			return fmt.Errorf("%w: --key wants TABLE=COL,COL, got %q", errUsage, k)
		}
		parts := strings.Split(cols, ",")
		for _, col := range parts {
			if strings.TrimSpace(col) == "" {
				return fmt.Errorf("%w: --key wants TABLE=COL,COL and one of the columns is empty, got %q",
					errUsage, k)
			}
		}
		req.Keys[table] = parts
	}
	return nil
}

// parseUnmask reads --unmask TABLE.COL=REASON.
//
// --unmask is the one safety rail the operator can pull, so a spelling that
// could never match a column is refused rather than stored: an opt-out that
// silently never applied looks exactly like one that did. The column is
// resolved against the source catalog later, in internal/core, which refuses a
// qualified name that names nothing; this is the half that can be checked
// without a database.
func parseUnmask(req *core.Request, raw *rawFlags) error {
	for _, u := range raw.unmask {
		col, reason, found := strings.Cut(u, "=")
		if !found || col == "" || strings.TrimSpace(reason) == "" {
			// The bare form is refused on purpose: an opt-out with no reason is
			// an opt-out nobody can review later (ARCHITECTURE.md section 8).
			return fmt.Errorf("%w: --unmask wants TABLE.COL=REASON, got %q", errUsage, u)
		}
		table, column, qualified := strings.Cut(col, ".")
		if !qualified || strings.TrimSpace(table) == "" || strings.TrimSpace(column) == "" {
			return fmt.Errorf(
				"%w: --unmask wants TABLE.COL=REASON, and the column must be qualified, got %q",
				errUsage, u)
		}
		if previous, duplicate := req.Unmask[col]; duplicate {
			return fmt.Errorf(
				"%w: --unmask %s given twice, with reasons %q and %q; one column has one reason",
				errUsage, col, previous, reason)
		}
		req.Unmask[col] = reason
	}
	return nil
}

// groupedUsage prints --help by stage rather than alphabetically.
func groupedUsage(groups []flagGroup) func(*cobra.Command) error {
	return func(cmd *cobra.Command) error {
		// cobra's help already printed Long; this function prints only what it
		// is for, which is the flags, grouped by the stage they act on.
		out := cmd.OutOrStderr()
		fmt.Fprintf(out, "Usage:\n  %s\n", cmd.UseLine())

		if cmd.HasAvailableSubCommands() {
			fmt.Fprintf(out, "\nCommands:\n")
			subs := cmd.Commands()
			sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
			for _, c := range subs {
				if c.IsAvailableCommand() {
					fmt.Fprintf(out, "  %-12s %s\n", c.Name(), c.Short)
				}
			}
		}

		for _, g := range groups {
			if !g.set.HasFlags() {
				continue
			}
			fmt.Fprintf(out, "\n%s:\n%s", g.title, g.set.FlagUsages())
		}

		if local := cmd.LocalNonPersistentFlags(); local.HasFlags() {
			fmt.Fprintf(out, "\nother:\n%s", local.FlagUsages())
		}
		return nil
	}
}
