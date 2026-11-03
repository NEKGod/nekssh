package main

import (
	"sync"
	"time"
)

// Only stage names and durations are logged; credentials and output are omitted.
func traceSSHStage(id, stage string) func(error) {
	started := time.Now()
	debugf("ssh timing id=%s stage=%s begin", id, stage)
	return func(err error) {
		debugf("ssh timing id=%s stage=%s elapsed=%s failed=%t", id, stage, time.Since(started).Round(time.Millisecond), err != nil)
	}
}

type sshConnectionTrace struct {
	started   time.Time
	firstRead sync.Once
	firstEmit sync.Once
}

func (t *sshConnectionTrace) output(id string, emit bool) {
	if t == nil {
		return
	}
	once, stage := &t.firstRead, "first_output_received"
	if emit {
		once, stage = &t.firstEmit, "first_output_sent_to_ui"
	}
	once.Do(func() {
		debugf("ssh timing id=%s stage=%s since_connect=%s", id, stage, time.Since(t.started).Round(time.Millisecond))
	})
}
