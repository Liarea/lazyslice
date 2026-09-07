// SPDX-License-Identifier: Apache-2.0

package core

import (
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
			c.sink.Send(e)
		}
	}()
	return c
}

// Send is event.Sink. It never returns an error and never panics on a closed
// channel, because close is called once, from Run's own defer, after every
// stage goroutine has been joined.
func (c *eventChannel) Send(e event.Event) {
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
