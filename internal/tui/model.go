// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The screen's default size, used until the first tea.WindowSizeMsg and in a
// test, which never gets one.
const (
	defaultWidth  = 96
	defaultHeight = 28
	// chromeLines is how many lines the header, the estimate, the notes, the
	// status line and the footer take, so that the table gets the rest.
	chromeLines = 9
	// minTableHeight keeps a row visible on a terminal too small to lay out.
	minTableHeight = 3
)

// model is the root Bubble Tea model: both screens, the request they build and
// the keybindings that build it.
//
// Every event.Event arrives as a tea.Msg, which is what makes internal/tui one
// more sink on the same channel as render.Lines rather than a second reader of
// the pipeline (ARCHITECTURE.md section 1 and section 7). The model reaches no
// stage: the only thing it produces is a core.Request, and every field it sets
// is a flag in ARCHITECTURE.md section 8.
type model struct {
	// base is the request the flags built, kept so that flags() can print the
	// difference the screens made as the flags that would make it again.
	base core.Request
	req  core.Request

	keys   keyMap
	screen screen

	reasons table.Model
	plan    table.Model

	rows     []reasonRow
	steps    []planRow
	estimate string
	notes    []string
	dropped  int

	prompt prompt
	status string

	showHelp      bool
	width, height int

	accepted bool
}

// prompt is the one-line answer a value-taking action asks for: the reason an
// opt-out records, or the count a --take, --cap or --depth wants.
type prompt struct {
	active  bool
	binding Binding
	label   string
	err     string
	input   lineInput
	column  ref.ColumnRef
	table   ref.TableRef
}

// newModel returns the model over the request the flags built.
//
// The request's maps and slices are copied rather than shared: the caller keeps
// the request it passed in, and the screens build on a copy, so leaving with
// "leave without running" leaves nothing behind.
func newModel(req core.Request) model {
	k := defaultKeyMap()
	m := model{
		base:   cloneRequest(req),
		req:    cloneRequest(req),
		keys:   k,
		width:  defaultWidth,
		height: defaultHeight,
	}
	m.reasons = table.New(table.WithFocused(true), table.WithKeyMap(tableKeyMap(k)))
	m.plan = table.New(table.WithFocused(true), table.WithKeyMap(tableKeyMap(k)))
	m.layout()
	m.refresh()
	return m
}

var _ tea.Model = model{}

// seed applies the events a run already produced, which is how the two screens
// are filled when the TUI is opened after the plan pass rather than during it.
//
// It is the same code path a live event takes through Update: one apply per
// event, one refresh at the end rather than one per event, because a schema
// with eight thousand columns would otherwise rebuild the table eight thousand
// times.
func (m model) seed(events []event.Event) model {
	for _, e := range events {
		m = m.apply(e)
	}
	m.refresh()
	return m
}

// setDropped records how many events the collector's bound refused, so that the
// header can say the screen is not the whole classification.
func (m model) setDropped(n int) model {
	m.dropped = n
	return m
}

// request is what the screens have built. It is the same struct cmd/lazyslice
// builds from flags, field for field.
func (m model) request() core.Request { return m.req }

// Init implements tea.Model.
func (model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case event.Event:
		m = m.apply(msg)
		m.refresh()
		return m, nil
	case tea.WindowSizeMsg:
		// A zero size is what arrives when the output is not a terminal, which
		// is every test and any caller that redirected stdout. Laying a table
		// out into it produces a screen with a header and no rows, so the
		// defaults are kept instead.
		if msg.Width > 0 && msg.Height > 0 {
			m.width, m.height = msg.Width, msg.Height
			m.layout()
			m.refresh()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.pressed(msg)
	default:
		return m, nil
	}
}

// View implements tea.Model.
//
// The two screens run in the alternate screen and leave nothing behind, because
// what survives the run is the line printer's transcript and, on leaving, the
// screen contents Run echoes into scrollback (ADR-002).
func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// apply folds one event into the screens.
func (m model) apply(e event.Event) model {
	switch e.Code {
	case codeColumnMasked, codeColumnCopied:
		m.rows = append(m.rows, reasonRow{
			Column: ref.ColumnRef{Table: e.Table, Column: e.Column},
			Masked: e.Code == codeColumnMasked,
			Reason: e.Args[event.ArgReason],
		})
	case core.CodePlanStep:
		mode, why := splitStep(e.Args[event.ArgReason])
		m.steps = append(m.steps, planRow{
			Table: e.Table,
			Rows:  e.Args[event.ArgCount],
			Mode:  mode,
			Why:   why,
		})
	case core.CodePlanEstimate:
		m.estimate = catalogueLine(e)
	case core.CodePlanPolymorphicInferred, core.CodePlanPolymorphic, core.CodePlanUnmapped:
		m.notes = append(m.notes, catalogueLine(e))
	default:
		// Every other code belongs to the transcript, which the line printer
		// behind the collector has already printed. A screen that showed all of
		// them would be a second, quieter transcript.
	}
	return m
}

// ---------- keys ----------

// pressed routes one key press.
func (m model) pressed(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.prompt.active {
		return m.promptKey(msg)
	}
	m.status = ""

	switch {
	case key.Matches(msg, m.keys.Cancel.Bind), key.Matches(msg, m.keys.Quit.Bind):
		m.accepted = false
		return m, tea.Quit
	case key.Matches(msg, m.keys.Accept.Bind):
		m.accepted = true
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help.Bind):
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(msg, m.keys.Switch.Bind):
		m.screen = m.screen.other()
		return m, nil
	}

	if b, ok := m.action(msg); ok {
		return m.act(b), nil
	}

	// What is left is navigation, and navigation belongs to the focused table.
	var cmd tea.Cmd
	switch m.screen {
	case screenPlan:
		m.plan, cmd = m.plan.Update(msg)
	default:
		m.reasons, cmd = m.reasons.Update(msg)
	}
	return m, cmd
}

// action finds the action binding for this key on this screen.
func (m model) action(msg tea.KeyPressMsg) (Binding, bool) {
	for _, b := range m.keys.actions() {
		if b.shows(m.screen) && key.Matches(msg, b.Bind) {
			return b, true
		}
	}
	return Binding{}, false
}

// act runs one action, or says why it is not available on this row.
//
// The switch is on the flag name rather than on a binding identity, because the
// flag *is* the action here: ADR-002's rule is that a screen offers nothing the
// command line cannot, and a case with no flag to switch on could not be
// written.
func (m model) act(b Binding) model {
	if !m.available(b) {
		m.status = b.Desc() + " — not available on this row"
		return m
	}
	switch b.Flag {
	case "unmask":
		return m.actUnmask(b)
	case "strict-schema":
		m.req.StrictSchema = !m.req.StrictSchema
		m.req.Explicit["strict-schema"] = m.req.StrictSchema
		m.status = "--strict-schema " + onOff(m.req.StrictSchema)
		return m
	case "root":
		row, ok := m.selectedStep()
		if !ok {
			return m
		}
		m.req.Root = row.Table.String()
		m.req.Explicit["root"] = true
		m.status = "--root " + m.req.Root + " — the plan is recomputed on the next run"
		return m
	case "skip-table":
		return m.actSkip()
	case "take":
		return m.open(b, "root rows"+inForce(m.req.Take), ref.ColumnRef{}, ref.TableRef{})
	case "depth":
		return m.open(b, "child depth"+inForce(m.req.Depth), ref.ColumnRef{}, ref.TableRef{})
	case "cap":
		row, ok := m.selectedStep()
		if !ok {
			return m
		}
		return m.open(b, "children per parent key on "+row.Table.String()+inForce(m.capOf(row.Table)),
			ref.ColumnRef{}, row.Table)
	default:
		return m
	}
}

// actUnmask toggles the per-column opt-out. Setting one asks for the reason
// --unmask TABLE.COL=REASON requires; clearing one asks nothing, because the
// absence of the flag is the absence of the opt-out.
func (m model) actUnmask(b Binding) model {
	row, ok := m.selectedReason()
	if !ok {
		return m
	}
	// The opt-out is deleted under the key it was stored with, which is not
	// necessarily the qualified spelling: the operator may have typed
	// --unmask TABLE.COL, and that is the same opt-out (unmaskKey).
	if stored, opted := unmaskKey(m.req.Unmask, row.Column); opted {
		delete(m.req.Unmask, stored)
		m.status = "masking restored on " + columnKey(row.Column)
		m.refresh()
		return m
	}
	return m.open(b, "reason for not masking "+columnKey(row.Column), row.Column, ref.TableRef{})
}

// actSkip toggles --skip-table on the selected table.
//
// Both branches refresh, because the skip is shown on the row it was made on:
// whyCell renders "skipped — ..." from the request, and a table that is not
// rebuilt still says the row is a child of its parent while the request says it
// moves no rows.
func (m model) actSkip() model {
	row, ok := m.selectedStep()
	if !ok {
		return m
	}
	name := row.Table.String()
	if i := slices.Index(m.req.SkipTables, name); i >= 0 {
		m.req.SkipTables = slices.Delete(slices.Clone(m.req.SkipTables), i, i+1)
		m.status = name + " is in the slice again"
		m.refresh()
		return m
	}
	m.req.SkipTables = append(slices.Clone(m.req.SkipTables), name)
	m.req.Explicit["skip-table"] = true
	m.status = "--skip-table " + name + " — schema only, no rows"
	m.refresh()
	return m
}

// available is the footer's state: a binding whose action cannot apply to the
// selected row is struck through rather than hidden (ADR-002).
func (m model) available(b Binding) bool {
	switch b.Flag {
	case "unmask":
		row, ok := m.selectedReason()
		if !ok {
			return false
		}
		_, opted := unmaskKey(m.req.Unmask, row.Column)
		return row.Masked || opted
	case "root":
		row, ok := m.selectedStep()
		return ok && !row.root()
	case "cap":
		row, ok := m.selectedStep()
		return ok && row.child()
	case "skip-table":
		row, ok := m.selectedStep()
		return ok && !row.root()
	default:
		return true
	}
}

// ---------- the prompt ----------

// open starts the one-line prompt for a value-taking action.
//
// The field starts empty and the label carries the value in force, rather than
// the field starting prefilled: a prefilled field means the first thing typed
// lands beside the old value rather than replacing it, and "50" typed over
// "500" becomes 50050 without a word of complaint.
func (m model) open(b Binding, label string, col ref.ColumnRef, tbl ref.TableRef) model {
	m.prompt = prompt{
		active:  true,
		binding: b,
		label:   label,
		input:   newLineInput(""),
		column:  col,
		table:   tbl,
	}
	return m
}

// inForce renders the value a count prompt would change, for its label.
func inForce(v int) string { return " (now " + strconv.Itoa(v) + ", enter alone keeps it)" }

// promptKey routes a key while the prompt is open. Cancel discards the answer;
// everything else that is not "confirm" is typed.
func (m model) promptKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel.Bind):
		m.prompt = prompt{}
		m.status = "discarded"
		return m, nil
	case key.Matches(msg, m.keys.Accept.Bind):
		return m.confirm(), nil
	default:
		m.prompt.err = ""
		m.prompt.input = m.prompt.input.Update(msg)
		return m, nil
	}
}

// confirm applies the prompt's answer to the request, or keeps the prompt open
// with the reason it could not.
func (m model) confirm() model {
	p := m.prompt
	value := strings.TrimSpace(p.input.Value())

	// An empty answer to a count prompt is "leave it as it is", which is what
	// the label offers. An empty reason is not: --unmask has no bare form.
	if value == "" && p.binding.Flag != "unmask" {
		m.prompt = prompt{}
		m.status = "unchanged"
		return m
	}

	switch p.binding.Flag {
	case "unmask":
		switch {
		case value == "":
			m.prompt.err = "a reason is required: an opt-out nobody can review later is not one"
			return m
		case strings.ContainsAny(value, "\n\r"):
			m.prompt.err = "a reason is one line"
			return m
		}
		m.req.Unmask[columnKey(p.column)] = value
		m.req.Explicit["unmask"] = true
		m.status = "--unmask " + columnKey(p.column) + "=" + value
	case "take":
		n, err := count(value)
		if err != nil {
			m.prompt.err = err.Error()
			return m
		}
		m.req.Take = n
		m.req.Explicit["take"] = true
		m.status = "--take " + value
	case "depth":
		n, err := count(value)
		if err != nil {
			m.prompt.err = err.Error()
			return m
		}
		m.req.Depth = n
		m.req.Explicit["depth"] = true
		m.status = "--depth " + value
	case "cap":
		n, err := count(value)
		if err != nil {
			m.prompt.err = err.Error()
			return m
		}
		// A per-table cap is not the global one, so Explicit["cap"] is left
		// alone: cmd/lazyslice's parseCaps records that key for the bare
		// --cap N form only, and setting it here would make lazyslice.yml's
		// global cap: silently ignored for every other table.
		m.req.TableCaps[p.table.String()] = n
		m.status = "--cap " + p.table.String() + "=" + value
	default:
	}

	m.prompt = prompt{}
	m.refresh()
	return m
}

// count reads a count flag's value the way cmd/lazyslice's checkCounts does: a
// zero or a negative is a refusal and not a silent default, because an int
// field cannot carry the difference between "unset" and "none".
func count(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, errBadCount
	}
	return n, nil
}

// errBadCount is the one message a count prompt can fail with.
var errBadCount = errors.New("a count of 1 or more")

// ---------- rows ----------

func (m model) selectedReason() (reasonRow, bool) {
	i := m.reasons.Cursor()
	if i < 0 || i >= len(m.rows) {
		return reasonRow{}, false
	}
	return m.rows[i], true
}

func (m model) selectedStep() (planRow, bool) {
	i := m.plan.Cursor()
	if i < 0 || i >= len(m.steps) {
		return planRow{}, false
	}
	return m.steps[i], true
}

// capOf is the cap in force on one table: the same choice internal/plan makes,
// which is the per-table --cap when there is one and the global --cap otherwise.
func (m model) capOf(t ref.TableRef) int {
	if n, ok := m.req.TableCaps[t.String()]; ok {
		return n
	}
	return m.req.Cap
}

// columnKey is the spelling the screens write an opt-out under: ref's own
// schema.table.column, which internal/core resolves to exactly this column.
func columnKey(c ref.ColumnRef) string { return c.String() }

// unmaskKey finds the key an existing opt-out on col is stored under, whatever
// spelling it arrived in.
//
// core.Request.Unmask is keyed by the string the operator typed, and --unmask
// takes TABLE.COL as well as SCHEMA.TABLE.COL (internal/core's resolveColumn
// resolves a bare table against the catalog). To the pipeline those two are one
// opt-out; to a map they are two keys. Looking one up by the qualified spelling
// alone made `--tui --unmask customer.email=...` show the column as copied with
// no opt-out on it and struck "u" through, so the screen refused to restore
// masking on a column it was about to copy in clear — and which spelling the
// operator used decided it.
//
// The keys are walked in order so that two spellings of one column resolve to
// the same one on every run.
func unmaskKey(unmask map[string]string, col ref.ColumnRef) (string, bool) {
	for _, name := range slices.Sorted(maps.Keys(unmask)) {
		if namesColumn(name, col) {
			return name, true
		}
	}
	return "", false
}

// namesColumn reports whether the spelling name is the column col.
//
// It admits what --unmask admits and no more: TABLE.COLUMN or
// SCHEMA.TABLE.COLUMN, each part optionally double-quoted, compared exactly
// (Postgres identifiers reach the catalog folded already, and internal/core
// compares them the same way). Anything else is left alone rather than guessed
// at, and internal/core refuses it with the name in the message.
func namesColumn(name string, col ref.ColumnRef) bool {
	parts, ok := splitQualified(name)
	if !ok {
		return false
	}
	switch len(parts) {
	case 2:
		return parts[0] == col.Table.Name && parts[1] == col.Column
	case 3:
		return parts[0] == col.Table.Schema && parts[1] == col.Table.Name && parts[2] == col.Column
	default:
		return false
	}
}

// splitQualified splits a dotted name into its parts, honouring double quotes,
// which is what internal/core's own splitQualified does to the same strings.
//
// It is a second copy of five lines rather than a shared one because
// internal/core keeps its splitter unexported, and this package builds a
// core.Request and reaches nothing else. The orchestrator has the note that
// keying core.Request.Unmask by a canonical column name would remove both
// copies and the guessing with them.
func splitQualified(s string) ([]string, bool) {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' && quoted && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quoted {
		return nil, false
	}
	return append(parts, cur.String()), true
}

// ---------- layout ----------

func (m *model) layout() {
	h := max(m.height-chromeLines, minTableHeight)
	m.reasons.SetHeight(h)
	m.plan.SetHeight(h)
	m.reasons.SetWidth(m.width)
	m.plan.SetWidth(m.width)
}

// refresh rebuilds both tables from the rows and the request, which is what
// makes an opt-out visible on the row it was made on.
func (m *model) refresh() {
	nameWidth := min(max(m.width*35/100, 18), 44)

	// The bubbles table clamps its cursor to the rows it holds, so setting the
	// columns of a table that has no rows yet leaves the cursor at -1 and every
	// action then reads "no row selected". The cursor is taken and put back
	// around the rebuild for that reason.
	reasonsCursor, planCursor := max(m.reasons.Cursor(), 0), max(m.plan.Cursor(), 0)

	m.reasons.SetColumns([]table.Column{
		{Title: "column", Width: nameWidth},
		{Title: "decision", Width: 8},
		{Title: "reason", Width: max(m.width-nameWidth-8-8, 12)},
	})
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		rows = append(rows, table.Row{columnKey(r.Column), m.decisionOf(r), r.Reason})
	}
	m.reasons.SetRows(rows)
	m.reasons.SetCursor(reasonsCursor)

	m.plan.SetColumns([]table.Column{
		{Title: "table", Width: nameWidth},
		{Title: "rows", Width: 9},
		{Title: "cap", Width: 5},
		{Title: "why", Width: max(m.width-nameWidth-9-5-10, 12)},
	})
	steps := make([]table.Row, 0, len(m.steps))
	for _, s := range m.steps {
		steps = append(steps, table.Row{s.Table.String(), s.Rows, m.capCell(s), m.whyCell(s)})
	}
	m.plan.SetRows(steps)
	m.plan.SetCursor(planCursor)
}

// decisionOf is the reasons screen's verdict cell: what the classifier decided,
// or that this run opts the column out of it.
func (m model) decisionOf(r reasonRow) string {
	if _, opted := unmaskKey(m.req.Unmask, r.Column); opted {
		return "opt-out"
	}
	if r.Masked {
		return "masked"
	}
	return "copied"
}

// capCell is the cap in force on a step, or "-" where none applies.
//
// The plan.step event does not carry Step.Cap, so the number is computed from
// the request the same way internal/plan computes it — the per-table --cap when
// there is one, the global --cap otherwise, and only on a child edge. That is
// the same arithmetic and not an estimate of it, which is why the column can
// say "cap" rather than "cap, probably".
func (m model) capCell(s planRow) string {
	if !s.child() {
		return "-"
	}
	return strconv.Itoa(m.capOf(s.Table))
}

// whyCell is the step's Why, with the skip this run has added marked on it.
func (m model) whyCell(s planRow) string {
	if slices.Contains(m.req.SkipTables, s.Table.String()) {
		return "skipped — " + s.Why
	}
	return s.Why
}

// ---------- the view ----------

var (
	titleStyle       = lipgloss.NewStyle().Bold(true)
	faintStyle       = lipgloss.NewStyle().Faint(true)
	unavailableStyle = lipgloss.NewStyle().Faint(true).Strikethrough(true)
)

func (m model) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.header()))
	b.WriteString("\n\n")
	if m.showHelp {
		b.WriteString(m.helpView())
	} else {
		b.WriteString(m.body())
	}
	b.WriteString("\n")
	if line := m.statusLine(); line != "" {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(m.footer())
	return b.String()
}

func (m model) header() string {
	switch m.screen {
	case screenPlan:
		return "lazyslice · plan · " + strconv.Itoa(len(m.steps)) + " tables" + m.droppedNote()
	default:
		return "lazyslice · reasons · " + strconv.Itoa(len(m.rows)) + " columns" + m.droppedNote()
	}
}

func (m model) droppedNote() string {
	if m.dropped == 0 {
		return ""
	}
	return " (" + strconv.Itoa(m.dropped) + " more not shown)"
}

func (m model) body() string {
	switch m.screen {
	case screenPlan:
		var b strings.Builder
		b.WriteString(m.plan.View())
		if m.estimate != "" {
			b.WriteString("\n" + faintStyle.Render(m.estimate))
		}
		for _, n := range m.notes {
			b.WriteString("\n" + faintStyle.Render(n))
		}
		return b.String()
	default:
		return m.reasons.View()
	}
}

func (m model) statusLine() string {
	switch {
	case m.prompt.active && m.prompt.err != "":
		return m.prompt.label + "\n" + m.prompt.input.View() + "\n" + m.prompt.err
	case m.prompt.active:
		return m.prompt.label + "\n" + m.prompt.input.View()
	case m.status != "":
		return faintStyle.Render(m.status)
	default:
		return ""
	}
}

// footerPart is one binding as the footer prints it: what to press, what it
// does, and whether it can be pressed on the selected row.
type footerPart struct {
	Key       string
	Desc      string
	Available bool
}

// footerParts is the footer, computed from state. A binding that is unavailable
// stays in the list and is struck through when it is rendered, rather than
// vanishing (ADR-002): a key that disappears reads as a key that never existed.
func (m model) footerParts() []footerPart {
	if m.prompt.active {
		return []footerPart{
			{Key: m.keys.Accept.Key(), Desc: "confirm", Available: true},
			{Key: m.keys.Cancel.Key(), Desc: "discard", Available: true},
		}
	}
	var out []footerPart
	for _, b := range m.keys.All() {
		if !b.shows(m.screen) {
			continue
		}
		out = append(out, footerPart{Key: b.Key(), Desc: b.Desc(), Available: m.available(b)})
	}
	return out
}

func (m model) footer() string {
	parts := m.footerParts()
	rendered := make([]string, 0, len(parts))
	for _, p := range parts {
		text := p.Key + " " + p.Desc
		if !p.Available {
			rendered = append(rendered, unavailableStyle.Render(text))
			continue
		}
		rendered = append(rendered, faintStyle.Render(p.Key)+" "+p.Desc)
	}
	return strings.Join(rendered, " · ")
}

// helpView is what "?" shows: every binding, what it does, and the flag it is.
//
// The flag column is the point of the screen. It is how an operator learns that
// the thing they just did in the TUI is a flag they can put in a script, and it
// is the same table TestEveryTUIActionHasFlag walks.
func (m model) helpView() string {
	var b strings.Builder
	b.WriteString("key      does                                          flag\n")
	for _, bind := range m.keys.All() {
		flag := "—"
		if bind.Flag != "" {
			flag = "--" + bind.Flag
		}
		b.WriteString(pad(bind.Key(), 8) + " " + pad(bind.Desc(), 45) + " " + flag + "\n")
	}
	b.WriteString("\n" + faintStyle.Render("every action here is a flag; nothing in these screens is only here"))
	return b.String()
}

func pad(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// ---------- what leaves ----------

// Flags is the command line that would make the same request without the TUI.
//
// It is the difference between the request the flags built and the request the
// screens built, spelled as flags. Run echoes it into scrollback on the way out,
// which is what turns "every TUI action is a flag" from a rule into something
// the operator can see and paste.
func (m model) flags() []string {
	var out []string
	if m.req.Root != m.base.Root && m.req.Root != "" {
		out = append(out, "--root "+m.req.Root)
	}
	if m.req.Take != m.base.Take {
		out = append(out, "--take "+strconv.Itoa(m.req.Take))
	}
	if m.req.Depth != m.base.Depth {
		out = append(out, "--depth "+strconv.Itoa(m.req.Depth))
	}
	if m.req.StrictSchema && !m.base.StrictSchema {
		out = append(out, "--strict-schema")
	}
	for _, t := range slices.Sorted(maps.Keys(m.req.TableCaps)) {
		if m.base.TableCaps[t] == m.req.TableCaps[t] {
			continue
		}
		out = append(out, "--cap "+t+"="+strconv.Itoa(m.req.TableCaps[t]))
	}
	for _, t := range m.req.SkipTables {
		if !slices.Contains(m.base.SkipTables, t) {
			out = append(out, "--skip-table "+t)
		}
	}
	for _, c := range slices.Sorted(maps.Keys(m.req.Unmask)) {
		if m.base.Unmask[c] == m.req.Unmask[c] {
			continue
		}
		out = append(out, "--unmask "+quoteArg(c+"="+m.req.Unmask[c]))
	}
	return out
}

// quoteArg quotes an argument a shell would otherwise split. An --unmask reason
// is a sentence, so it is quoted more often than not.
func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t\"'\\$`") {
		return strconv.Quote(s)
	}
	return s
}

// Transcript is what Run echoes into scrollback on the way out: the screen that
// was open, its estimate, and the flags that would rebuild the request.
//
// ADR-002's reason for the line printer being the default is that the
// transcript is the artefact — it survives the run and pastes into a compliance
// ticket — so the two screens must not be the one part of a run that leaves
// nothing behind.
func (m model) transcript() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.body())
	b.WriteString("\n")
	if flags := m.flags(); len(flags) > 0 {
		// The flags the screens set, not a whole command line: the source, the
		// target and everything else the operator already typed are still
		// theirs, and printing half a command as though it were all of it is
		// the kind of line that gets pasted and then fails.
		b.WriteString("flags these screens set: " + strings.Join(flags, " ") + "\n")
	}
	return b.String()
}

// ---------- the request ----------

// cloneRequest copies a request deeply enough that the screens cannot write
// through the caller's maps and slices.
func cloneRequest(r core.Request) core.Request {
	out := r
	out.TableCaps = maps.Clone(r.TableCaps)
	out.Keys = maps.Clone(r.Keys)
	out.Unmask = maps.Clone(r.Unmask)
	out.Explicit = maps.Clone(r.Explicit)
	out.SkipTables = slices.Clone(r.SkipTables)
	if out.TableCaps == nil {
		out.TableCaps = map[string]int{}
	}
	if out.Keys == nil {
		out.Keys = map[string][]string{}
	}
	if out.Unmask == nil {
		out.Unmask = map[string]string{}
	}
	if out.Explicit == nil {
		out.Explicit = map[string]bool{}
	}
	return out
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
