// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
)

// eventBuffer is ARCHITECTURE.md section 7's bounded channel: "core.Run sends
// into a bounded channel of 256. Progress events are dropped when the channel is
// full, with the drop counted; every other kind blocks."
const eventBuffer = 256

// eventChannel is that channel, with the one sink of the run on the far end.
//
// It exists so that a slow renderer cannot back-pressure the pipeline into
// deadlock and cannot silently stall it either. A Progress event is a count that
// will be superseded by the next one, so dropping it costs a redraw; every other
// kind is a decision, a refusal or a stage transition, and losing one of those
// would make the transcript a different account of the run from the yml. So
// Progress is dropped when the buffer is full and everything else blocks, which
// is the rule section 7 states.
//
// The drops are counted rather than ignored: a transcript missing progress lines
// says why, once, at the end (CodeProgressDropped). run.sink is this type in
// every run, so no stage and no renderer can tell the difference.
type eventChannel struct {
	sink  event.Sink
	ch    chan event.Event
	done  chan struct{}
	drops atomic.Int64
	once  sync.Once

	// mu guards closed: Send takes it for reading (RLock), close takes it for
	// writing (Lock) around setting closed and closing ch, so a Send already
	// past the closed check cannot land on ch after it closes, and a Send that
	// starts after close sees closed and never touches ch at all (2026-09-14
	// review of T-FAILUX, finding 2). Without this, a panic that unwinds past
	// move's ordinary joins (run.go's <-r.extractDone, <-r.transformDone) could
	// let a stage goroutine's next event.Sink.Send race Run's own defer closing
	// ch — a second, unrecoverable panic on a closed channel, in a goroutine
	// guardedExecute's recover cannot reach. run.close now joins the stage
	// goroutines before this ever runs (belt); this is the suspenders for any
	// sender this package does not itself track the lifetime of.
	mu     sync.RWMutex
	closed bool

	// sinkPanic holds the first panic recovered from a call into sink.Send, if
	// there ever was one. A sink is caller-supplied render code (internal/render,
	// or a TUI feed) running on the one drain goroutine below, which
	// guardedExecute's own recover cannot see (it only wraps root.ExecuteContext
	// on the main goroutine) — without a recover of its own here, a panic in a
	// sink printed Go's raw, unredacted goroutine trace and exited 2 whatever
	// --debug said (2026-09-14 review of T-FAILUX, finding 1).
	sinkPanic atomic.Value // panicError
}

// newEventChannel starts the drain goroutine. Every event the run sends from
// here on reaches the sink from that one goroutine, so a sink needs no locking of
// its own — which is what lets render.Lines hold a bufio.Writer.
func newEventChannel(sink event.Sink) *eventChannel {
	c := &eventChannel{
		sink: sink,
		ch:   make(chan event.Event, eventBuffer),
		done: make(chan struct{}),
	}
	go func() {
		defer close(c.done)
		for e := range c.ch {
			c.sendToSink(e)
		}
	}()
	return c
}

// sendToSink is the drain goroutine's only call into caller-supplied code,
// recovered on its own so that a panic in a sink cannot reach the top of the
// process unguarded (see sinkPanic). Only the first panic is kept: a sink that
// panics repeatedly would otherwise overwrite the one Run reports with a later,
// less useful one.
func (c *eventChannel) sendToSink(e event.Event) {
	defer func() {
		if v := recover(); v != nil {
			c.sinkPanic.CompareAndSwap(nil, panicError{val: v, stack: debug.Stack()})
		}
	}()
	c.sink.Send(e)
}

// panicked returns the sink's first recovered panic, or nil if it never
// panicked, so Run can surface it through the normal Stop path instead of the
// process crashing under it.
func (c *eventChannel) panicked() error {
	if v, ok := c.sinkPanic.Load().(panicError); ok {
		return v
	}
	return nil
}

// Send is event.Sink. It never panics on a closed channel: closed is checked
// under mu, which close also takes before it closes ch, so the two cannot
// interleave (see mu's doc).
func (c *eventChannel) Send(e event.Event) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return
	}
	if e.Kind == event.Progress {
		select {
		case c.ch <- e:
		default:
			c.drops.Add(1)
		}
		return
	}
	c.ch <- e
}

// close drains the channel, waits for the sink to have seen everything, and
// reports the drops. It is idempotent.
func (c *eventChannel) close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		close(c.ch)
		<-c.done
		if n := c.drops.Load(); n > 0 {
			c.sink.Send(event.Event{
				At: time.Now(), Stage: event.Emit, Kind: event.Warn, Code: CodeProgressDropped,
				Args: event.Args{event.ArgCount: strconv.FormatInt(n, 10)},
			})
		}
	})
}

// panicError is a panic recovered off a goroutine guardedExecute's own recover
// cannot see (cmd/lazyslice/main.go), converted into an ordinary error so it
// can travel the same Stop path every other failure does. Its Stack method is
// picked up structurally by cmd/lazyslice's --debug reporting (errors.As
// against an unexported interface), so a recovered panic gets the same
// stack-under-debug treatment as one guardedExecute recovers directly,
// whichever package built it.
type panicError struct {
	val   any
	stack []byte
}

// Error names the *type* of the recovered value and never the value itself,
// and that is the 2026-09-15 red team's finding against THREAT_MODEL.md T4.
//
// T4's control is stated as "event.Event has no free-form string field", which
// is true and does not cover the error egress. A panic message is a free-form
// string by construction, and a masker — or anything under one: pgx's encoding,
// the phonenumbers parser, a JSON walker — that panics with the offending input
// in its message wrote that input to stderr at any verbosity, through this
// Error() and through cmd/lazyslice's reportPanic, both of which formatted the
// recovered value with %v. A custom masker doing it deliberately is T12's
// central case; a library doing it by accident is the same leak with nobody to
// blame.
//
// The value is still reachable, behind the flag that already exists for exactly
// this decision: PanicValue below is read by cmd/lazyslice under
// --show-row-values-in-errors. The stack trace stays behind --debug as before,
// which is a separate question — a stack frame carries no row value.
func (p panicError) Error() string { return "panic: " + PanicSummary(p.val) }
func (p panicError) Stack() []byte { return p.stack }

// PanicValue is the recovered value, for the one caller allowed to print it:
// cmd/lazyslice's error egress under --show-row-values-in-errors. It is
// matched structurally there, the way Stack already is.
func (p panicError) PanicValue() any { return p.val }

// PanicSummary describes a recovered panic value without quoting it: its type,
// and for an error the fact that it is one. It is exported because
// cmd/lazyslice's own recover (guardedExecute) needs the same redaction and may
// not reproduce it — one rule, one function (THREAT_MODEL.md T4).
//
// A string panic is the dangerous one and is the case this exists for:
// panic("masker blew up on \"victim.canary@bigcorp.com\"") is a production
// value formatted into a message, and %v prints it whole.
func PanicSummary(v any) string {
	if v == nil {
		return "a nil value"
	}
	if _, ok := v.(error); ok {
		return fmt.Sprintf("an error of type %T (its message is withheld because it may quote a value: "+
			"--show-row-values-in-errors)", v)
	}
	return fmt.Sprintf("a value of type %T (it is withheld because it may quote a value: "+
		"--show-row-values-in-errors)", v)
}
