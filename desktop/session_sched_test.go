package main

import (
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestFinishSessionConnectSignalsReadyBeforeFollowUps(t *testing.T) {
	app := NewApp()
	const id = "session-ready"
	app.sessions[id] = &desktopSession{client: &ssh.Client{}}
	dials := 0
	previous := openSSH
	openSSH = func(req ConnectionRequest, config *ssh.ClientConfig) (*ssh.Client, error) {
		dials++
		return nil, errors.New("should not dial an existing session")
	}
	t.Cleanup(func() { openSSH = previous })

	release := make(chan struct{})
	started := make(chan struct{})
	var mu sync.Mutex
	var order []string
	app.connectFollowUps = []func(){
		func() {
			close(started)
			<-release
			mu.Lock()
			order = append(order, "stats")
			mu.Unlock()
		},
		func() {
			<-release
			mu.Lock()
			order = append(order, "files")
			mu.Unlock()
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- app.Connect(ConnectionRequest{ID: id, Host: "10.1.1.1", Username: "root", Port: 22})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connect waited for stats or directory work")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("follow-up work did not start after ready")
	}
	mu.Lock()
	if len(order) != 0 {
		t.Fatalf("follow-up finished before release: %v", order)
	}
	mu.Unlock()
	if dials != 0 {
		t.Fatalf("existing session dialed %d times", dials)
	}
	close(release)
}

func TestSameTargetReusesSSHClient(t *testing.T) {
	pool := &clientPool{}
	dials := 0
	dial := func() (*ssh.Client, error) {
		dials++
		return &ssh.Client{}, nil
	}
	key := clientShareKey(ConnectionRequest{Username: "root", Host: "10.1.1.1", Port: 22})
	first, shared, err := pool.Acquire(key, dial)
	if err != nil {
		t.Fatal(err)
	}
	if shared || first == nil || dials != 1 {
		t.Fatalf("first dial shared=%v dials=%d", shared, dials)
	}
	second, shared, err := pool.Acquire(key, dial)
	if err != nil {
		t.Fatal(err)
	}
	if !shared || second != first || dials != 1 {
		t.Fatalf("second connection dialed again shared=%v dials=%d same=%v", shared, dials, second == first)
	}
	other := clientShareKey(ConnectionRequest{Username: "root", Host: "10.2.2.2", Port: 22})
	if _, shared, err = pool.Acquire(other, dial); err != nil || shared || dials != 2 {
		t.Fatalf("different host shared=%v dials=%d err=%v", shared, dials, err)
	}
}

func TestStatsRefreshSkipsOverlappingCommand(t *testing.T) {
	if allowStatsRefresh(true) {
		t.Fatal("in-flight stats refresh was allowed to start")
	}
	if !allowStatsRefresh(false) {
		t.Fatal("idle stats refresh was refused")
	}
	gate := &statsRunGate{}
	held := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		_ = gate.Run("server-a", func() error {
			close(held)
			<-release
			return nil
		})
		close(finished)
	}()
	<-held
	remoteCommands := 0
	err := gate.Run("server-a", func() error {
		remoteCommands++
		return nil
	})
	if !errors.Is(err, errStatsInFlight) {
		t.Fatalf("overlap error = %v", err)
	}
	if remoteCommands != 0 {
		t.Fatalf("overlapping refresh issued %d remote commands", remoteCommands)
	}
	close(release)
	<-finished
	if err = gate.Run("server-a", func() error {
		remoteCommands++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if remoteCommands != 1 {
		t.Fatalf("command count = %d", remoteCommands)
	}
}
