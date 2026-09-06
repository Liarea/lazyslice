// SPDX-License-Identifier: Apache-2.0

// Package tui holds the two Bubble Tea screens --tui opens: the classification
// reasons screen and the plan screen (ADR-002).
//
// The TUI is a thin layer. Every action it offers must be reachable by a CLI
// flag first, and it builds the same core.Request the flags do; it never reaches
// a stage. Until these screens land, "?" at a prompt prints the same table
// through the line printer and $PAGER, which is the behaviour the v1 cut line
// records.
//
// Scaffold status: no-op. Run returns ErrNotImplemented.
package tui

import (
	"context"
	"errors"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Liarea/lazyslice/internal/event"
)

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("tui: not implemented")

// Model is the root Bubble Tea model. Each event.Event arrives as a tea.Msg, so
// the TUI is one more sink on the same channel as the line printer.
type Model struct {
	// Reasons is the classification screen: one row per column with its
	// category, confidence and reason.
	Reasons table.Model
	// Plan is the plan screen: the steps, the cycles and the estimate, scrolled.
	Plan viewport.Model
}

// Init implements tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View implements tea.Model.
func (Model) View() tea.View { return tea.NewView("") }

var _ tea.Model = Model{}

// Run enters the TUI and forwards every event to it as a tea.Msg.
//
// Scaffold status: no-op.
func Run(_ context.Context, _ <-chan event.Event) error { return ErrNotImplemented }
