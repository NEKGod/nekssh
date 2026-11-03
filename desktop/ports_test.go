package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestListeningPortsParsesIPv4IPv6UDPAndSharedSockets(t *testing.T) {
	items := parseListeningPorts(`tcp LISTEN 0 128 0.0.0.0:8080 0.0.0.0:* users:(("node",pid=42,fd=5),("node",pid=43,fd=5))
tcp LISTEN 0 128 [::]:22 [::]:* users:(("sshd",pid=9,fd=3))
udp UNCONN 0 0 127.0.0.53:53 0.0.0.0:*
tcp LISTEN 0 128 *:8080 *:* users:(("node",pid=42,fd=6))
tcp LISTEN 0 128 *:8080 *:* users:(("node",pid=42,fd=6))
---NEKSSH-PROCESSES---
9 root /usr/sbin/sshd -D
42 app node /srv/server.js --port 8080
43 app node worker.js
`)
	if len(items) != 5 {
		t.Fatalf("got %d entries: %+v", len(items), items)
	}
	if items[0].Address != "::" || items[0].PID != 9 || items[0].User != "root" {
		t.Fatalf("IPv6/metadata: %+v", items[0])
	}
	if items[1].Protocol != "udp" || items[1].PID != 0 {
		t.Fatalf("UDP without permission: %+v", items[1])
	}
	found := false
	for _, item := range items {
		if item.PID == 42 && strings.Contains(item.Command, "--port 8080") {
			found = true
		}
	}
	if !found {
		t.Fatal("full process command not attached")
	}
}

func TestTunnelTargets(t *testing.T) {
	for address, want := range map[string]string{"*": "127.0.0.1:80", "0.0.0.0": "127.0.0.1:80", "::": "[::1]:80", "192.168.1.9": "192.168.1.9:80"} {
		got, err := tunnelTarget("tcp", address, 80)
		if err != nil || got != want {
			t.Fatalf("%s => %s %v", address, got, err)
		}
	}
	for _, item := range []struct {
		protocol, address string
		port              int
	}{{"udp", "127.0.0.1", 53}, {"tcp", "bad address", 80}, {"tcp", "127.0.0.1", 0}, {"tcp", "127.0.0.1", 65536}} {
		if _, err := tunnelTarget(item.protocol, item.address, item.port); err == nil {
			t.Fatalf("accepted invalid target: %+v", item)
		}
	}
}

// Exercise the real SSH direct-tcpip transport and TCP listeners, not mocks of StartPortTunnel.
func TestPortTunnelTransferReuseConflictAndCleanup(t *testing.T) {
	remote, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	go func() {
		for {
			conn, err := remote.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); io.Copy(conn, conn) }()
		}
	}()
	port := remote.Addr().(*net.TCPAddr).Port
	client := portTestSSHClient(t)
	session := &desktopSession{client: client, done: make(chan struct{})}
	app := NewApp()
	session.app = app
	app.sessions["test"] = session
	t.Cleanup(session.stop)
	tunnel, err := app.StartPortTunnel("test", "tcp", "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if tunnel.LocalAddress == remote.Addr().String() {
		t.Fatal("did not select free port when preferred port occupied")
	}
	if !strings.HasPrefix(tunnel.LocalAddress, "127.0.0.1:") {
		t.Fatalf("not loopback: %s", tunnel.LocalAddress)
	}
	again, err := app.StartPortTunnel("test", "tcp", "127.0.0.1", port)
	if err != nil || again != tunnel {
		t.Fatalf("duplicate mapping: %+v %v", again, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, e := net.DialTimeout("tcp", tunnel.LocalAddress, time.Second)
			if e != nil {
				t.Error(e)
				return
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			want := fmt.Sprintf("concurrent-%d", i)
			if _, e = io.WriteString(conn, want); e != nil {
				t.Error(e)
				return
			}
			data := make([]byte, len(want))
			_, e = io.ReadFull(conn, data)
			if e != nil || string(data) != want {
				t.Errorf("forwarded %q %v", data, e)
			}
		}(i)
	}
	wg.Wait()
	active, err := net.DialTimeout("tcp", tunnel.LocalAddress, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	active.SetDeadline(time.Now().Add(3 * time.Second))
	io.WriteString(active, "ready")
	data := make([]byte, 5)
	if _, err = io.ReadFull(active, data); err != nil {
		t.Fatal(err)
	}
	if err = app.StopPortTunnel("test", tunnel.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = active.Read(data); err == nil {
		t.Fatal("stop left active connection open")
	}
	if list, _ := app.PortTunnels("test"); len(list) != 0 {
		t.Fatal("stopped tunnel still listed")
	}
	newTunnel, err := app.StartPortTunnel("test", "tcp", "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	session.stop()
	if conn, err := net.DialTimeout("tcp", newTunnel.LocalAddress, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("disconnect left listener open")
	}
}

func TestSavedListenRulesShareTunnelManagement(t *testing.T) {
	app := NewApp()
	client := portTestSSHClient(t)
	first := &desktopSession{client: client, done: make(chan struct{}), app: app}
	second := &desktopSession{client: client, done: make(chan struct{}), app: app}
	app.sessions["first"], app.sessions["second"] = first, second
	t.Cleanup(first.stop)
	t.Cleanup(second.stop)
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()
	rule := LocalForward{ListenPort: port, TargetHost: "127.0.0.1", TargetPort: 8080, Enabled: true}
	for _, session := range []*desktopSession{first, second} {
		if note, err := app.registerForward(session, "127.0.0.1", rule); err != nil || note != "" {
			t.Fatalf("register saved rule: %q %v", note, err)
		}
	}
	items, err := app.PortTunnels("first")
	if err != nil || len(items) != 1 {
		t.Fatalf("saved Listen rule absent from panel: %+v %v", items, err)
	}
	// Reuse must not probe/recreate the saved mapping, even if its service is offline.
	reused, err := app.StartPortTunnel("first", "tcp", "0.0.0.0", 8080)
	if err != nil || reused != items[0] || len(app.forwards) != 1 {
		t.Fatalf("one-click did not reuse saved mapping: %+v %v", reused, err)
	}
	if err := app.StopPortTunnel("first", reused.ID); err != nil {
		t.Fatal(err)
	}
	if remaining, _ := app.PortTunnels("first"); len(remaining) != 0 {
		t.Fatal("stopped saved rule still listed")
	}
	if remaining, _ := app.PortTunnels("second"); len(remaining) != 1 {
		t.Fatal("stopping one session removed another session's shared rule")
	}
	if conn, err := net.DialTimeout("tcp", reused.LocalAddress, time.Second); err != nil {
		t.Fatal("shared listener closed prematurely", err)
	} else {
		conn.Close()
	}
	second.stop()
	if conn, err := net.DialTimeout("tcp", reused.LocalAddress, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("last disconnect left saved listener open")
	}
}

func portTestSSHClient(t *testing.T) *ssh.Client {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		raw, e := listener.Accept()
		if e != nil {
			return
		}
		server, channels, requests, e := ssh.NewServerConn(raw, config)
		if e != nil {
			raw.Close()
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			if incoming.ChannelType() != "direct-tcpip" {
				incoming.Reject(ssh.UnknownChannelType, "unsupported")
				continue
			}
			var target struct {
				Host       string
				Port       uint32
				Origin     string
				OriginPort uint32
			}
			if e = ssh.Unmarshal(incoming.ExtraData(), &target); e != nil {
				incoming.Reject(ssh.ConnectionFailed, "invalid target")
				continue
			}
			remote, e := net.DialTimeout("tcp", net.JoinHostPort(target.Host, strconv.Itoa(int(target.Port))), time.Second)
			if e != nil {
				incoming.Reject(ssh.ConnectionFailed, e.Error())
				continue
			}
			channel, r, e := incoming.Accept()
			if e != nil {
				remote.Close()
				continue
			}
			go ssh.DiscardRequests(r)
			go func() {
				defer remote.Close()
				defer channel.Close()
				done := make(chan struct{}, 1)
				go func() { io.Copy(remote, channel); done <- struct{}{} }()
				go func() { io.Copy(channel, remote); done <- struct{}{} }()
				<-done
			}()
		}
	}()
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}
