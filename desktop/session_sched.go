package main

import (
	"errors"
	"sync"
)

// errStatsInFlight is returned when a stats command is already running for the session.
var errStatsInFlight = errors.New("服务器资源刷新仍在进行")

// statsRunGate lets one remote stats command run per session.
type statsRunGate struct {
	mu   sync.Mutex
	busy map[string]bool
}

// Begin reports whether this session may start a remote stats command.
func (g *statsRunGate) Begin(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.busy == nil {
		g.busy = map[string]bool{}
	}
	if !allowStatsRefresh(g.busy[id]) {
		return false
	}
	g.busy[id] = true
	return true
}

// End marks the remote stats command finished.
func (g *statsRunGate) End(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.busy, id)
}

// Run runs command only when no stats command for id is already in flight.
func (g *statsRunGate) Run(id string, command func() error) error {
	if !g.Begin(id) {
		return errStatsInFlight
	}
	defer g.End(id)
	return command()
}

// allowStatsRefresh is the overlap decision: a refresh already in flight must not start another.
func allowStatsRefresh(inFlight bool) bool {
	return !inFlight
}

// finishSessionConnect signals the session ready before any follow-up work.
func finishSessionConnect(markReady func(), followUps []func()) {
	if markReady != nil {
		markReady()
	}
	for _, follow := range followUps {
		follow := follow
		go follow()
	}
}
