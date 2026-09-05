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
// Scaffold status: every flag below is registered and parsed, and every stage
// behind them is a documented no-op. A run therefore announces the nine stages
// and exits with ExitInternal. Nothing connects to a database, and nothing is
// written.
package main

import (
	"context"
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
		subcommand(ctx, "introspect", "Read the source catalog and print it", req, raw, stdout),
		subcommand(ctx, "classify", "Classify every column and print the reasons", req, raw, stdout),
		subcommand(ctx, "plan", "Print the subset plan and stop", req, raw, stdout),
		subcommand(ctx, "verify", "Re-run the checks against an existing target", req, raw, stdout),
		subcommand(ctx, "doctor", "Print what lazyslice can see and what it cannot prove", req, raw, stdout),
		newVersionCmd(stdout),
	)

	return root
}

// report maps an error to an exit code and prints it once. Every mapping here
// is an ADR-005 code; anything unmapped is ExitInternal, because a code that
// means "something went wrong" must not be confused with one a CI job branches
// on.
func report(stderr io.Writer, err error, showValues bool) int {
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(stderr, "lazyslice: interrupted")
		return ExitInterrupted
	case errors.Is(err, pipeline.ErrNotImplemented):
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		fmt.Fprintln(stderr, "  this build is the phase 3 scaffold: the flag surface is real, the stages are not")
		return ExitInternal
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "lazyslice: %s\n", renderSafe(err, showValues))
		return ExitUsage
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
	name, short string,
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
			if name == "plan" {
				req.PlanOnly = true
			}
			if err := finish(cmd, req, raw); err != nil {
				return err
			}
			_, err := core.Run(ctx, *req, sinkFor(*req, stdout))
			return err
		},
	}
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

// finish parses the TABLE=VALUE flags into the request and fills in what the
// process knows: the working directory.
func finish(_ *cobra.Command, req *core.Request, raw *rawFlags) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: cannot read the working directory: %w", errUsage, err)
	}
	req.Workdir = wd

	for _, c := range raw.caps {
		table, value, found := strings.Cut(c, "=")
		if !found {
			n, err := strconv.Atoi(c)
			if err != nil {
				return fmt.Errorf("%w: --cap wants N or TABLE=N, got %q", errUsage, c)
			}
			req.Cap = n
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: --cap wants N or TABLE=N, got %q", errUsage, c)
		}
		req.TableCaps[table] = n
	}

	for _, k := range raw.keys {
		table, cols, found := strings.Cut(k, "=")
		if !found || table == "" || cols == "" {
			return fmt.Errorf("%w: --key wants TABLE=COL,COL, got %q", errUsage, k)
		}
		req.Keys[table] = strings.Split(cols, ",")
	}

	for _, u := range raw.unmask {
		col, reason, found := strings.Cut(u, "=")
		if !found || col == "" || reason == "" {
			// The bare form is refused on purpose: an opt-out with no reason is
			// an opt-out nobody can review later (ARCHITECTURE.md section 8).
			return fmt.Errorf("%w: --unmask wants TABLE.COL=REASON, got %q", errUsage, u)
		}
		// An unqualified name can never match a column reference, so it would
		// be an opt-out that silently never applied: indistinguishable, in the
		// output, from one that did. Masking is the one rail the operator can
		// pull, so the shape is checked here rather than shrugged at.
		if table, column, qualified := strings.Cut(col, "."); !qualified || table == "" || column == "" {
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

	req.SkipTables = append(req.SkipTables, raw.skipTables...)
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
