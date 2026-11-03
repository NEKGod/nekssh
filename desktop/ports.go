package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ListeningPort struct {
	Protocol, Address      string
	Port, PID              int
	Process, User, Command string
}

type PortTunnel struct {
	ID, RemoteAddress, LocalAddress string
	RemotePort                      int
	CreatedAt                       int64
}

type portTunnel struct {
	info        PortTunnel
	listener    net.Listener
	mu          sync.Mutex
	closed      bool
	connections map[net.Conn]bool
}

const listeningPortsCommand = `LC_ALL=C ss -H -l -n -t -u -p || exit; printf '\n---NEKSSH-PROCESSES---\n'; LC_ALL=C ps -eo pid=,user=,args=`

func (a *App) ListeningPorts(id string) ([]ListeningPort, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	output, err := runOnServer(session, listeningPortsCommand)
	if err != nil {
		return nil, fmt.Errorf("读取监听端口失败: %w", err)
	}
	return parseListeningPorts(output), nil
}

var socketProcessPattern = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

func parseListeningPorts(output string) []ListeningPort {
	sections := strings.SplitN(output, "---NEKSSH-PROCESSES---", 2)
	metadata := map[int][]string{}
	if len(sections) == 2 {
		for _, line := range strings.Split(sections[1], "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			pid, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			metadata[pid] = []string{fields[1], strings.Join(fields[2:], " ")}
		}
	}
	ports := make([]ListeningPort, 0)
	seen := map[string]bool{}
	for _, line := range strings.Split(sections[0], "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || (fields[0] != "tcp" && fields[0] != "udp") {
			continue
		}
		endpoint := fields[4]
		colon := strings.LastIndex(endpoint, ":")
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(endpoint[colon+1:])
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		address := strings.Trim(endpoint[:colon], "[]")
		matches := socketProcessPattern.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			matches = [][]string{{"", "", "0"}}
		}
		for _, match := range matches {
			pid, _ := strconv.Atoi(match[2])
			key := fmt.Sprintf("%s/%s/%d", fields[0], endpoint, pid)
			if seen[key] {
				continue
			}
			seen[key] = true
			item := ListeningPort{Protocol: fields[0], Address: address, Port: port, PID: pid, Process: match[1]}
			if detail := metadata[pid]; len(detail) == 2 {
				item.User, item.Command = detail[0], detail[1]
			}
			ports = append(ports, item)
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Port != ports[j].Port {
			return ports[i].Port < ports[j].Port
		}
		if ports[i].Protocol != ports[j].Protocol {
			return ports[i].Protocol < ports[j].Protocol
		}
		if ports[i].Address != ports[j].Address {
			return ports[i].Address < ports[j].Address
		}
		return ports[i].PID < ports[j].PID
	})
	return ports
}

func tunnelTarget(protocol, address string, port int) (string, error) {
	if protocol != "tcp" {
		return "", errors.New("SSH 一键穿透仅支持 TCP 端口")
	}
	if port < 1 || port > 65535 {
		return "", errors.New("端口必须在 1–65535 之间")
	}
	switch address {
	case "", "*", "0.0.0.0":
		address = "127.0.0.1"
	case "::":
		address = "::1"
	}
	if net.ParseIP(address) == nil {
		return "", errors.New("监听地址不是有效 IP 地址")
	}
	return net.JoinHostPort(address, strconv.Itoa(port)), nil
}

func (a *App) StartPortTunnel(id, protocol, address string, port int) (PortTunnel, error) {
	target, err := tunnelTarget(protocol, address, port)
	if err != nil {
		return PortTunnel{}, err
	}
	session, err := a.getSession(id)
	if err != nil {
		return PortTunnel{}, err
	}
	a.forwardMu.Lock()
	if existing := a.findForwardLocked(session, target); existing != nil {
		info := existing.tunnel.info
		a.forwardMu.Unlock()
		return info, nil
	}
	a.forwardMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe, err := session.client.DialContext(ctx, "tcp", target)
	if err != nil {
		return PortTunnel{}, fmt.Errorf("服务器端口 %s 无法连接: %w", target, err)
	}
	probe.Close()
	a.forwardMu.Lock()
	defer a.forwardMu.Unlock()
	select {
	case <-session.done:
		return PortTunnel{}, errors.New("SSH 会话已断开")
	default:
	}
	if existing := a.findForwardLocked(session, target); existing != nil {
		return existing.tunnel.info, nil
	}
	listener, err := listenPortTunnel(port)
	if err != nil {
		return PortTunnel{}, fmt.Errorf("创建本地监听失败: %w", err)
	}
	return a.addForwardLocked(session, listener, target, port).tunnel.info, nil
}

// Saved Listen rules and one-click tunnels share listeners, lifecycle and status.
func (a *App) addForwardLocked(session *desktopSession, listener net.Listener, target string, port int) *sharedForward {
	key := listener.Addr().String()
	tunnel := &portTunnel{info: PortTunnel{ID: key, RemoteAddress: target, RemotePort: port, LocalAddress: key, CreatedAt: time.Now().UnixMilli()}, listener: listener, connections: map[net.Conn]bool{}}
	forward := &sharedForward{listener: listener, tunnel: tunnel, target: target, sessions: map[*desktopSession]bool{session: true}}
	a.forwards[key] = forward
	go a.forwardLocal(forward)
	return forward
}

func (a *App) findForwardLocked(session *desktopSession, target string) *sharedForward {
	for _, forward := range a.forwards {
		if forward.target == target && forward.sessions[session] {
			return forward
		}
	}
	return nil
}

func listenPortTunnel(port int) (net.Listener, error) {
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err == nil {
		return listener, nil
	}
	if !isAddrInUse(err) {
		return nil, err
	}
	return net.Listen("tcp4", "127.0.0.1:0")
}

func (a *App) PortTunnels(id string) ([]PortTunnel, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	a.forwardMu.Lock()
	defer a.forwardMu.Unlock()
	items := make([]PortTunnel, 0)
	for _, forward := range a.forwards {
		if forward.sessions[session] {
			items = append(items, forward.tunnel.info)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RemoteAddress < items[j].RemoteAddress })
	return items, nil
}

func (a *App) StopPortTunnel(id, tunnelID string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	a.forwardMu.Lock()
	defer a.forwardMu.Unlock()
	if forward := a.forwards[tunnelID]; forward != nil && forward.sessions[session] {
		delete(forward.sessions, session)
		if len(forward.sessions) == 0 {
			forward.tunnel.stop()
			delete(a.forwards, tunnelID)
		}
	}
	return nil
}

func (t *portTunnel) track(conn net.Conn) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		conn.Close()
		return false
	}
	t.connections[conn] = true
	return true
}

func (t *portTunnel) untrack(conn net.Conn) {
	conn.Close()
	t.mu.Lock()
	delete(t.connections, conn)
	t.mu.Unlock()
}

func (t *portTunnel) serve(dial func(context.Context) (net.Conn, error)) {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		if !t.track(local) {
			continue
		}
		go func() {
			defer t.untrack(local)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remote, err := dial(ctx)
			if err != nil {
				return
			}
			if !t.track(remote) {
				return
			}
			defer t.untrack(remote)
			bridgeConnections(local, remote)
		}()
	}
}

func (t *portTunnel) stop() {
	t.mu.Lock()
	t.closed = true
	connections := make([]net.Conn, 0, len(t.connections))
	for conn := range t.connections {
		connections = append(connections, conn)
	}
	t.mu.Unlock()
	t.listener.Close()
	for _, conn := range connections {
		conn.Close()
	}
}
