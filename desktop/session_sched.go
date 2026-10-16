package main

import (
	"errors"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
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

// clientPool keeps one SSH client for repeated connections to the same target.
type clientPool struct {
	mu      sync.Mutex
	entries map[string]*pooledClient
}

type pooledClient struct {
	client *ssh.Client
	refs   int
}

// Acquire returns a live client for key. shared is true when an existing client was reused.
func (p *clientPool) Acquire(key string, dial func() (*ssh.Client, error)) (*ssh.Client, bool, error) {
	p.mu.Lock()
	if entry := p.entries[key]; entry != nil && entry.client != nil && entry.refs > 0 {
		entry.refs++
		client := entry.client
		p.mu.Unlock()
		return client, true, nil
	}
	p.mu.Unlock()
	client, err := dial()
	if err != nil {
		return nil, false, err
	}
	if client == nil {
		return nil, false, errors.New("SSH 连接失败")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = map[string]*pooledClient{}
	}
	if entry := p.entries[key]; entry != nil && entry.client != nil && entry.refs > 0 {
		entry.refs++
		go client.Close()
		return entry.client, true, nil
	}
	p.entries[key] = &pooledClient{client: client, refs: 1}
	return client, false, nil
}

// Release drops one use of key and closes the client when nothing else uses it.
func (p *clientPool) Release(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := p.entries[key]
	if entry == nil {
		return
	}
	entry.refs--
	if entry.refs > 0 {
		return
	}
	if entry.client != nil {
		entry.client.Close()
	}
	delete(p.entries, key)
}

func clientShareKey(req ConnectionRequest) string {
	return strings.Join([]string{req.Username, req.Host, strconv.Itoa(req.Port), req.ProxyType, req.ProxyHost, strconv.Itoa(req.ProxyPort)}, "\x00")
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
