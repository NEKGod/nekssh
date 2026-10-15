package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/sftp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"
	"golang.org/x/sys/windows"
)

type App struct {
	ctx         context.Context
	mu          sync.RWMutex
	forwardMu   sync.Mutex
	sessions         map[string]*desktopSession
	pendingKeys      map[string]ssh.PublicKey
	forwards         map[string]*sharedForward
	statsRuns        statsRunGate
	connectFollowUps []func()
}

type desktopSession struct {
	host        string
	client      *ssh.Client
	session     *ssh.Session
	stdin       io.WriteCloser
	sftp        *sftp.Client
	close       sync.Once
	notify      sync.Once
	done        chan struct{}
	app         *App
	forwardKeys []string
}

type sharedForward struct {
	listener net.Listener
	target   string
	sessions map[*desktopSession]bool
}

type LocalForward struct {
	Name, ListenAddress, TargetHost string
	ListenPort, TargetPort          int
	Enabled                         bool
}

type ConnectionRequest struct {
	ID, Host, Username, Password, PrivateKey, Passphrase string
	ProxyType, ProxyHost, ProxyUsername, ProxyPassword   string
	ListenAddress, TargetHost                            string
	Port, Cols, Rows, ProxyPort, ListenPort, TargetPort  int
	Forwards                                             []LocalForward
}

type FileEntry struct {
	Name, Path, Modified, Mode, Owner, Group string
	Size                                     int64
	Directory                                bool
}

type ServerStats struct {
	Host                    string
	CPUPercent              float64
	MemoryUsed, MemoryTotal uint64
	SwapUsed, SwapTotal     uint64
	DiskUsed, DiskTotal     uint64
}

type ServerProcess struct {
	PID                  int
	CPU                  float64
	Memory               float64
	Command, User, Ports string
}

type ServerPartition struct {
	Device, Mount string
	Used, Total   uint64
	Percent       float64
}

type FileHistoryEntry struct {
	ID, Time, Kind string
	Size           int
}

type fileHistorySnapshot struct {
	Time, Kind, Content string
}

type TransferProgress struct {
	ID, Direction, Name, Status, LocalPath string
	Transferred, Total                     int64
}

type IdentityOption struct {
	ID   int
	Name string
}

type OwnershipOptions struct {
	Users, Groups []IdentityOption
}

type PrivateKeyFile struct {
	Path, Content string
}

type RemoteImage struct {
	Name, Data string
	Size       int64
}

type SystemdServiceRequest struct {
	Name, Description, Executable, Arguments, User, WorkingDirectory string
	EnableNow                                                        bool
}

type SystemdService struct {
	Name, Active, Sub, Description string
}

type progressWriter struct {
	app                            *App
	id, direction, name, localPath string
	total, transferred             int64
	lastEmit                       time.Time
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	writer.transferred += int64(len(data))
	if time.Since(writer.lastEmit) >= 100*time.Millisecond || writer.transferred >= writer.total {
		writer.emit("进行中")
	}
	return len(data), nil
}

func (writer *progressWriter) emit(status string) {
	writer.lastEmit = time.Now()
	runtime.EventsEmit(writer.app.ctx, "transfer:progress", TransferProgress{ID: writer.id, Direction: writer.direction, Name: writer.name, Status: status, LocalPath: writer.localPath, Transferred: writer.transferred, Total: writer.total})
}

func NewApp() *App {
	return &App{sessions: make(map[string]*desktopSession), pendingKeys: make(map[string]ssh.PublicKey), forwards: make(map[string]*sharedForward)}
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) SelectPrivateKeyFile() (PrivateKeyFile, error) {
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 SSH 私钥文件", Filters: []runtime.FileFilter{{DisplayName: "SSH 私钥", Pattern: "*.pem;*.key;id_rsa;id_ed25519;id_ecdsa;*"}}})
	if err != nil || filePath == "" {
		return PrivateKeyFile{}, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return PrivateKeyFile{}, err
	}
	return PrivateKeyFile{Path: filePath, Content: string(data)}, nil
}

func (a *App) ProtectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data := []byte(value)
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, 0, &output); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	encrypted := append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (a *App) UnprotectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, 0, &output); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	plain := append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	return string(plain), nil
}
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, session := range a.sessions {
		session.stop()
	}
}

func (a *App) Connect(req ConnectionRequest) error {
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Username) == "" {
		return errors.New("连接名称、主机和用户名不能为空")
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if req.Port < 1 || req.Port > 65535 {
		return errors.New("SSH 端口无效")
	}
	if req.Cols < 20 {
		req.Cols = 100
	}
	if req.Rows < 5 {
		req.Rows = 30
	}
	if a.reusableClient(req.ID) {
		finishSessionConnect(func() { a.signalConnected(req.ID) }, a.connectFollowUps)
		return nil
	}
	auth, err := desktopAuth(req)
	if err != nil {
		return err
	}
	callback, err := desktopHostKeyCallback()
	if err != nil {
		return err
	}
	client, err := openSSH(req, &ssh.ClientConfig{User: req.Username, Auth: auth, HostKeyCallback: callback, Timeout: 10 * time.Second})
	if err != nil {
		return friendlyDesktopError(err)
	}
	shell, err := client.NewSession()
	if err != nil {
		client.Close()
		return err
	}
	if err = shell.RequestPty("xterm-256color", req.Rows, req.Cols, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		shell.Close()
		client.Close()
		return err
	}
	stdin, _ := shell.StdinPipe()
	stdout, _ := shell.StdoutPipe()
	stderr, _ := shell.StderrPipe()
	if err = shell.Shell(); err != nil {
		shell.Close()
		client.Close()
		return err
	}
	sftpClient, _ := sftp.NewClient(client)
	entry := &desktopSession{host: req.Host, client: client, session: shell, stdin: stdin, sftp: sftpClient, done: make(chan struct{}), app: a}
	var notes []string
	if req.ListenPort > 0 && req.TargetHost != "" && req.TargetPort > 0 {
		req.Forwards = append(req.Forwards, LocalForward{Name: "本地转发", ListenAddress: req.ListenAddress, ListenPort: req.ListenPort, TargetHost: req.TargetHost, TargetPort: req.TargetPort, Enabled: true})
	}
	for _, forward := range req.Forwards {
		if !forward.Enabled {
			continue
		}
		listenAddress := forward.ListenAddress
		if listenAddress == "" {
			listenAddress = "127.0.0.1"
		}
		note, registerErr := a.registerForward(entry, listenAddress, forward)
		if registerErr != nil {
			notes = append(notes, fmt.Sprintf("转发规则 %q 未启动: %v", forward.Name, registerErr))
			continue
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	a.mu.Lock()
	if old := a.sessions[req.ID]; old != nil {
		old.stop()
	}
	a.sessions[req.ID] = entry
	a.mu.Unlock()
	go a.stream(req.ID, entry, stdout)
	go a.stream(req.ID, entry, stderr)
	go a.keepalive(req.ID, entry)
	finishSessionConnect(func() { a.signalConnected(req.ID) }, a.connectFollowUps)
	if len(notes) > 0 && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "forward:notice", req.ID, strings.Join(notes, "；"))
	}
	return nil
}

func (a *App) reusableClient(id string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	session := a.sessions[id]
	return session != nil && session.client != nil
}

func (a *App) signalConnected(id string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "ssh:connected", id)
	}
}

var openSSH = dialSSH

func dialSSH(req ConnectionRequest, config *ssh.ClientConfig) (*ssh.Client, error) {
	target := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	connection, err := dialConnection(req, target)
	if err != nil {
		return nil, err
	}
	sshConn, channels, requests, err := ssh.NewClientConn(connection, target, config)
	if err != nil {
		connection.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, channels, requests), nil
}

func dialConnection(req ConnectionRequest, target string) (net.Conn, error) {
	var connection net.Conn
	var err error
	switch strings.ToLower(strings.TrimSpace(req.ProxyType)) {
	case "", "direct":
		connection, err = net.DialTimeout("tcp", target, 10*time.Second)
	case "socks5":
		var auth *proxy.Auth
		if req.ProxyUsername != "" {
			auth = &proxy.Auth{User: req.ProxyUsername, Password: req.ProxyPassword}
		}
		dialer, dialerErr := proxy.SOCKS5("tcp", net.JoinHostPort(req.ProxyHost, strconv.Itoa(req.ProxyPort)), auth, &net.Dialer{Timeout: 10 * time.Second})
		if dialerErr != nil {
			return nil, dialerErr
		}
		connection, err = dialer.Dial("tcp", target)
	case "http":
		connection, err = dialHTTPProxy(req, target)
	default:
		return nil, errors.New("不支持的代理类型")
	}
	return connection, err
}

func dialHTTPProxy(req ConnectionRequest, target string) (net.Conn, error) {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(req.ProxyHost, strconv.Itoa(req.ProxyPort)), 10*time.Second)
	if err != nil {
		return nil, err
	}
	headers := ""
	if req.ProxyUsername != "" {
		token := base64.StdEncoding.EncodeToString([]byte(req.ProxyUsername + ":" + req.ProxyPassword))
		headers = "Proxy-Authorization: Basic " + token + "\r\n"
	}
	if _, err = fmt.Fprintf(connection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", target, target, headers); err != nil {
		connection.Close()
		return nil, err
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
	if err != nil {
		connection.Close()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		connection.Close()
		return nil, fmt.Errorf("HTTP 代理连接失败: %s", response.Status)
	}
	return connection, nil
}

func (a *App) registerForward(session *desktopSession, listenAddress string, rule LocalForward) (string, error) {
	if rule.ListenPort <= 0 || rule.ListenPort > 65535 {
		return "", fmt.Errorf("监听端口无效")
	}
	key := net.JoinHostPort(listenAddress, strconv.Itoa(rule.ListenPort))
	target := net.JoinHostPort(rule.TargetHost, strconv.Itoa(rule.TargetPort))
	a.forwardMu.Lock()
	defer a.forwardMu.Unlock()
	if existing := a.forwards[key]; existing != nil {
		if existing.target != target {
			return "", fmt.Errorf("%s 已转发到 %s", key, existing.target)
		}
		existing.sessions[session] = true
		session.forwardKeys = append(session.forwardKeys, key)
		return "", nil
	}
	listener, err := net.Listen("tcp", key)
	if err != nil {
		if isAddrInUse(err) {
			return fmt.Sprintf("转发 %s：%s 已由其他程序监听，本实例不再绑定", rule.Name, key), nil
		}
		return "", err
	}
	forward := &sharedForward{listener: listener, target: target, sessions: map[*desktopSession]bool{session: true}}
	a.forwards[key] = forward
	session.forwardKeys = append(session.forwardKeys, key)
	go a.forwardLocal(forward)
	return "", nil
}

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

func (a *App) forwardLocal(forward *sharedForward) {
	for {
		local, err := forward.listener.Accept()
		if err != nil {
			return
		}
		a.forwardMu.Lock()
		sessions := make([]*desktopSession, 0, len(forward.sessions))
		for session := range forward.sessions {
			sessions = append(sessions, session)
		}
		a.forwardMu.Unlock()
		var remote net.Conn
		for _, session := range sessions {
			remote, err = session.client.Dial("tcp", forward.target)
			if err == nil {
				break
			}
		}
		if remote == nil {
			local.Close()
			continue
		}
		go bridgeConnections(local, remote)
	}
}

func (a *App) releaseForwards(session *desktopSession) {
	a.forwardMu.Lock()
	defer a.forwardMu.Unlock()
	for _, key := range session.forwardKeys {
		forward := a.forwards[key]
		if forward == nil {
			continue
		}
		delete(forward.sessions, session)
		if len(forward.sessions) == 0 {
			forward.listener.Close()
			delete(a.forwards, key)
		}
	}
	session.forwardKeys = nil
}

func bridgeConnections(left, right net.Conn) {
	defer left.Close()
	defer right.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(left, right); done <- struct{}{} }()
	go func() { _, _ = io.Copy(right, left); done <- struct{}{} }()
	<-done
}

func (a *App) stream(id string, session *desktopSession, reader io.Reader) {
	buffer := make([]byte, 8192)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			runtime.EventsEmit(a.ctx, "ssh:output", id, string(buffer[:n]))
		}
		if err != nil {
			break
		}
	}
	session.notify.Do(func() { runtime.EventsEmit(a.ctx, "ssh:closed", id) })
}

func (a *App) keepalive(id string, session *desktopSession) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-session.done:
			return
		case <-ticker.C:
			if _, _, err := session.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				session.notify.Do(func() { runtime.EventsEmit(a.ctx, "ssh:closed", id) })
				session.stop()
				return
			}
		}
	}
}

func (a *App) Input(id, data string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	_, err = io.WriteString(session.stdin, data)
	return err
}

func (a *App) CommandHistory(id string) ([]string, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sshSession.Close()
	command := `if command -v bash >/dev/null 2>&1; then bash -lc 'set -o history; history -r; history' 2>/dev/null; elif command -v zsh >/dev/null 2>&1; then zsh -lc 'fc -rl 1' 2>/dev/null; else cat "$HOME/.sh_history" 2>/dev/null; fi`
	output, err := sshSession.CombinedOutput(command)
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("读取命令历史失败: %w", err)
	}
	linePattern := regexp.MustCompile(`^\s*[0-9]+\s+`)
	seen := map[string]bool{}
	history := make([]string, 0, 500)
	lines := strings.Split(string(output), "\n")
	for index := len(lines) - 1; index >= 0 && len(history) < 500; index-- {
		line := strings.TrimSpace(linePattern.ReplaceAllString(lines[index], ""))
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		history = append(history, line)
	}
	return history, nil
}
func (a *App) Resize(id string, cols, rows int) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if cols < 20 || rows < 5 {
		return errors.New("终端尺寸无效")
	}
	return session.session.WindowChange(rows, cols)
}

func (a *App) ServerStats(id string) (ServerStats, error) {
	var stats ServerStats
	err := a.statsRuns.Run(id, func() error {
		session, err := a.getSession(id)
		if err != nil {
			return err
		}
		command := `set -e
host=$(hostname 2>/dev/null || uname -n 2>/dev/null || printf unknown)
read cpu user nice system idle iowait irq softirq steal guest guestnice < /proc/stat
total1=$((user+nice+system+idle+iowait+irq+softirq+steal))
idle1=$((idle+iowait))
sleep 0.2
read cpu user nice system idle iowait irq softirq steal guest guestnice < /proc/stat
total2=$((user+nice+system+idle+iowait+irq+softirq+steal))
idle2=$((idle+iowait))
cpu_pct=$(awk -v total=$((total2-total1)) -v idle=$((idle2-idle1)) 'BEGIN{if(total>0) printf "%.1f", (total-idle)*100/total; else print "0"}')
printf 'host=%s\n' "$host"
printf 'cpu=%.1f\n' "$cpu_pct"
free -b | awk '/^Mem:/ {print "mem_total="$2; print "mem_used="$3} /^Swap:/ {print "swap_total="$2; print "swap_used="$3}'
df -B1 / | awk 'NR==2 {print "disk_total="$2; print "disk_used="$3}'
`
		sshSession, err := session.client.NewSession()
		if err != nil {
			return err
		}
		defer sshSession.Close()
		output, err := sshSession.CombinedOutput(command)
		if err != nil {
			return fmt.Errorf("读取服务器资源失败: %w", err)
		}
		stats = parseServerStats(string(output))
		return nil
	})
	return stats, err
}

func parseServerStats(output string) ServerStats {
	stats := ServerStats{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "host":
			stats.Host = value
		case "cpu":
			stats.CPUPercent, _ = strconv.ParseFloat(value, 64)
		case "mem_used":
			stats.MemoryUsed, _ = strconv.ParseUint(value, 10, 64)
		case "mem_total":
			stats.MemoryTotal, _ = strconv.ParseUint(value, 10, 64)
		case "swap_used":
			stats.SwapUsed, _ = strconv.ParseUint(value, 10, 64)
		case "swap_total":
			stats.SwapTotal, _ = strconv.ParseUint(value, 10, 64)
		case "disk_used":
			stats.DiskUsed, _ = strconv.ParseUint(value, 10, 64)
		case "disk_total":
			stats.DiskTotal, _ = strconv.ParseUint(value, 10, 64)
		}
	}
	return stats
}

func (a *App) ServerProcesses(id string) ([]ServerProcess, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(`LC_ALL=C ps -eo pid=,pcpu=,pmem=,user=,comm= --sort=-pcpu | head -n 100; printf '\n---PORTS---\n'; ss -lntupH 2>/dev/null || true`)
	if err != nil {
		return nil, fmt.Errorf("读取服务器进程失败: %w", err)
	}
	sections := strings.SplitN(string(output), "---PORTS---", 2)
	portsByPID := map[int]map[string]bool{}
	if len(sections) == 2 {
		pidPattern := regexp.MustCompile(`pid=(\d+)`)
		portPattern := regexp.MustCompile(`:([0-9]+)$`)
		for _, line := range strings.Split(sections[1], "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			portMatch := portPattern.FindStringSubmatch(fields[4])
			if len(portMatch) != 2 {
				continue
			}
			for _, match := range pidPattern.FindAllStringSubmatch(line, -1) {
				pid, _ := strconv.Atoi(match[1])
				if portsByPID[pid] == nil {
					portsByPID[pid] = map[string]bool{}
				}
				portsByPID[pid][portMatch[1]] = true
			}
		}
	}
	processes := make([]ServerProcess, 0, 100)
	for _, line := range strings.Split(sections[0], "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		cpu, cpuErr := strconv.ParseFloat(fields[1], 64)
		memory, memoryErr := strconv.ParseFloat(fields[2], 64)
		if pidErr != nil || cpuErr != nil || memoryErr != nil {
			continue
		}
		ports := make([]string, 0, len(portsByPID[pid]))
		for port := range portsByPID[pid] {
			ports = append(ports, port)
		}
		sort.Strings(ports)
		processes = append(processes, ServerProcess{PID: pid, CPU: cpu, Memory: memory, User: fields[3], Command: strings.Join(fields[4:], " "), Ports: strings.Join(ports, ",")})
	}
	return processes, nil
}

func (a *App) SignalProcess(id string, pid int, signal string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	allowed := map[string]bool{"TERM": true, "KILL": true, "STOP": true, "CONT": true}
	signal = strings.ToUpper(strings.TrimSpace(signal))
	if pid <= 0 || !allowed[signal] {
		return errors.New("进程操作参数无效")
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(fmt.Sprintf("kill -%s -- %d", signal, pid))
	if err != nil {
		return fmt.Errorf("操作进程失败: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *App) ServerPartitions(id string) ([]ServerPartition, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(`LC_ALL=C df -P -B1 -x tmpfs -x devtmpfs 2>/dev/null | tail -n +2`)
	if err != nil {
		return nil, fmt.Errorf("读取服务器分区失败: %w", err)
	}
	partitions := make([]ServerPartition, 0, 8)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		total, totalErr := strconv.ParseUint(fields[1], 10, 64)
		used, usedErr := strconv.ParseUint(fields[2], 10, 64)
		percentValue, percentErr := strconv.ParseFloat(strings.TrimSuffix(fields[4], "%"), 64)
		if totalErr != nil || usedErr != nil || percentErr != nil {
			continue
		}
		partitions = append(partitions, ServerPartition{Device: fields[0], Mount: strings.Join(fields[5:], " "), Used: used, Total: total, Percent: percentValue})
	}
	return partitions, nil
}

func (a *App) Disconnect(id string) {
	a.mu.Lock()
	if session := a.sessions[id]; session != nil {
		session.stop()
		delete(a.sessions, id)
	}
	a.mu.Unlock()
}

func (a *App) ListFiles(id, remotePath string) ([]FileEntry, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	if session.sftp == nil {
		return nil, errors.New("目标服务器不支持 SFTP")
	}
	if remotePath == "" {
		remotePath = "."
	}
	resolved, err := session.sftp.RealPath(remotePath)
	if err != nil {
		return nil, err
	}
	items, err := session.sftp.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, len(items))
	for _, item := range items {
		entry := FileEntry{Name: item.Name(), Path: path.Join(resolved, item.Name()), Size: item.Size(), Directory: item.IsDir(), Modified: item.ModTime().Format(time.RFC3339), Mode: item.Mode().String()}
		if stat, ok := item.Sys().(*sftp.FileStat); ok {
			entry.Owner, entry.Group = fmt.Sprint(stat.UID), fmt.Sprint(stat.GID)
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func (a *App) ReadRemoteFile(id, remotePath string) (string, error) {
	session, err := a.getSession(id)
	if err != nil {
		return "", err
	}
	if session.sftp == nil {
		return "", errors.New("目标服务器不支持 SFTP")
	}
	info, err := session.sftp.Stat(remotePath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("目录不能使用代码编辑器打开")
	}
	const maxEditorFileSize = 2 * 1024 * 1024
	if info.Size() > maxEditorFileSize {
		return "", errors.New("文件超过 2 MB，请下载后编辑")
	}
	file, err := session.sftp.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxEditorFileSize+1))
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("二进制文件不能使用代码编辑器打开")
	}
	return string(data), nil
}

func (a *App) ReadRemoteImage(id, remotePath string) (RemoteImage, error) {
	session, err := a.getSession(id)
	if err != nil {
		return RemoteImage{}, err
	}
	if session.sftp == nil {
		return RemoteImage{}, errors.New("目标服务器不支持 SFTP")
	}
	info, err := session.sftp.Stat(remotePath)
	if err != nil {
		return RemoteImage{}, err
	}
	if info.IsDir() {
		return RemoteImage{}, errors.New("目录不能作为图片打开")
	}
	const maxImageSize = 20 * 1024 * 1024
	if info.Size() > maxImageSize {
		return RemoteImage{}, errors.New("图片超过 20 MB，请下载后查看")
	}
	file, err := session.sftp.Open(remotePath)
	if err != nil {
		return RemoteImage{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		return RemoteImage{}, err
	}
	return RemoteImage{Name: path.Base(remotePath), Size: info.Size(), Data: base64.StdEncoding.EncodeToString(data)}, nil
}

func (a *App) ReadRemotePDF(id, remotePath string) (RemoteImage, error) {
	session, err := a.getSession(id)
	if err != nil {
		return RemoteImage{}, err
	}
	if session.sftp == nil {
		return RemoteImage{}, errors.New("目标服务器不支持 SFTP")
	}
	info, err := session.sftp.Stat(remotePath)
	if err != nil {
		return RemoteImage{}, err
	}
	if info.IsDir() {
		return RemoteImage{}, errors.New("目录不能作为 PDF 打开")
	}
	const maxPDFSize = 40 * 1024 * 1024
	if info.Size() > maxPDFSize {
		return RemoteImage{}, errors.New("PDF 超过 40 MB，请下载后查看")
	}
	file, err := session.sftp.Open(remotePath)
	if err != nil {
		return RemoteImage{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPDFSize+1))
	if err != nil {
		return RemoteImage{}, err
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return RemoteImage{}, errors.New("这不是有效的 PDF 文件")
	}
	return RemoteImage{Name: path.Base(remotePath), Size: info.Size(), Data: base64.StdEncoding.EncodeToString(data)}, nil
}

func (a *App) IsELF(id, remotePath string) (bool, error) {
	session, err := a.getSession(id)
	if err != nil {
		return false, err
	}
	file, err := session.sftp.Open(remotePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err = io.ReadFull(file, header); err != nil {
		return false, nil
	}
	return bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}), nil
}

func (a *App) CreateSystemdService(id string, req SystemdServiceRequest) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(strings.TrimSpace(req.Name), ".service")
	validName := regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)
	if !validName.MatchString(name) || name == "" {
		return errors.New("服务名称无效")
	}
	if !strings.HasPrefix(req.Executable, "/") || strings.ContainsAny(req.Executable+req.Arguments+req.User+req.WorkingDirectory, "\r\n") {
		return errors.New("服务配置包含无效内容")
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = name
	}
	workingDirectory := strings.TrimSpace(req.WorkingDirectory)
	if workingDirectory == "" {
		workingDirectory = path.Dir(req.Executable)
	}
	userLine := ""
	if strings.TrimSpace(req.User) != "" {
		userLine = "User=" + strings.TrimSpace(req.User) + "\n"
	}
	unit := fmt.Sprintf("[Unit]\nDescription=%s\nAfter=network.target\n\n[Service]\nType=simple\n%sWorkingDirectory=%s\nExecStart=\"%s\"%s\nRestart=on-failure\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n", description, userLine, workingDirectory, strings.ReplaceAll(req.Executable, `"`, `\"`), func() string {
		if strings.TrimSpace(req.Arguments) != "" {
			return " " + strings.TrimSpace(req.Arguments)
		}
		return ""
	}())
	unitPath := "/etc/systemd/system/" + name + ".service"
	file, err := session.sftp.OpenFile(unitPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("写入服务文件失败，需要 root 权限: %w", err)
	}
	if _, err = io.WriteString(file, unit); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	action := "systemctl daemon-reload"
	if req.EnableNow {
		action += " && systemctl enable --now " + name + ".service"
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(action)
	if err != nil {
		return fmt.Errorf("加载服务失败: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *App) SystemdServices(id string) ([]SystemdService, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(`LC_ALL=C systemctl list-units --type=service --all --no-legend --no-pager`)
	if err != nil {
		return nil, fmt.Errorf("读取 systemd 服务失败: %w", err)
	}
	services := []SystemdService{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		services = append(services, SystemdService{Name: fields[0], Active: fields[2], Sub: fields[3], Description: strings.Join(fields[4:], " ")})
	}
	return services, nil
}

func (a *App) SystemdServiceAction(id, name, action string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.@-]+\.service$`).MatchString(name) {
		return errors.New("服务名称无效")
	}
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}
	if !allowed[action] {
		return errors.New("服务操作无效")
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput("systemctl " + action + " " + name)
	if err != nil {
		return fmt.Errorf("服务操作失败: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *App) SystemdServicePath(id, name string) (string, error) {
	session, err := a.getSession(id)
	if err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.@-]+\.service$`).MatchString(name) {
		return "", errors.New("服务名称无效")
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput("systemctl show -p FragmentPath --value " + name)
	if err != nil {
		return "", fmt.Errorf("读取服务配置路径失败: %s", strings.TrimSpace(string(output)))
	}
	filePath := strings.TrimSpace(string(output))
	if filePath == "" {
		return "", errors.New("该服务没有可编辑的 Unit 文件")
	}
	return filePath, nil
}

func (a *App) SystemdDaemonReload(id string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput("systemctl daemon-reload")
	if err != nil {
		return fmt.Errorf("重新加载 systemd 配置失败: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *App) SaveRemoteFile(id, remotePath, content string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	if previous, readErr := readRemoteEditorFile(session, remotePath); readErr == nil && previous != content {
		if historyErr := saveFileHistory(session.host, remotePath, previous, "保存前版本"); historyErr != nil {
			return fmt.Errorf("保存编辑历史失败: %w", historyErr)
		}
	}
	file, err := session.sftp.OpenFile(remotePath, os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, content)
	return err
}

func (a *App) FileHistory(id, remotePath string) ([]FileHistoryEntry, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	directory, err := fileHistoryDirectory(session.host, remotePath)
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []FileHistoryEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]FileHistoryEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(directory, file.Name()))
		if readErr != nil {
			continue
		}
		var snapshot fileHistorySnapshot
		if json.Unmarshal(data, &snapshot) != nil {
			continue
		}
		kind := snapshot.Kind
		if kind == "" {
			kind = "历史版本"
		}
		entries = append(entries, FileHistoryEntry{ID: strings.TrimSuffix(file.Name(), ".json"), Time: snapshot.Time, Kind: kind, Size: len(snapshot.Content)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Time > entries[j].Time })
	return entries, nil
}

func (a *App) ReadFileHistory(id, remotePath, historyID string) (string, error) {
	session, err := a.getSession(id)
	if err != nil {
		return "", err
	}
	if historyID == "" || strings.ContainsAny(historyID, `/\\`) {
		return "", errors.New("历史记录编号无效")
	}
	directory, err := fileHistoryDirectory(session.host, remotePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(directory, historyID+".json"))
	if err != nil {
		return "", err
	}
	var snapshot fileHistorySnapshot
	if err = json.Unmarshal(data, &snapshot); err != nil {
		return "", err
	}
	return snapshot.Content, nil
}

func (a *App) RecordFileHistory(id, remotePath, content, kind string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(kind) == "" {
		kind = "手动记录"
	}
	return saveFileHistory(session.host, remotePath, content, kind)
}

func readRemoteEditorFile(session *desktopSession, remotePath string) (string, error) {
	file, err := session.sftp.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	return string(data), err
}

func saveFileHistory(host, remotePath, content, kind string) error {
	directory, err := fileHistoryDirectory(host, remotePath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	now := time.Now()
	snapshot := fileHistorySnapshot{Time: now.Format(time.RFC3339), Kind: kind, Content: content}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(directory, now.Format("20060102T150405.000000000")+".json"), data, 0600); err != nil {
		return err
	}
	files, _ := os.ReadDir(directory)
	if len(files) > 30 {
		sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
		for _, file := range files[30:] {
			_ = os.Remove(filepath.Join(directory, file.Name()))
		}
	}
	return nil
}

func fileHistoryDirectory(host, remotePath string) (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(host + "\x00" + remotePath))
	return filepath.Join(config, "NekSSH", "history", hex.EncodeToString(hash[:])), nil
}

func (a *App) DownloadFile(id, remotePath, operationID string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	info, err := session.sftp.Stat(remotePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("请选择一个文件")
	}
	destination, err := chooseSavePath("保存远程文件", path.Base(remotePath))
	if err != nil || destination == "" {
		if err != nil {
			debugf("download dialog %s: %v", remotePath, err)
		}
		return err
	}
	debugf("download %s -> %s size=%d", remotePath, destination, info.Size())
	source, err := session.sftp.Open(remotePath)
	if err != nil {
		return err
	}
	defer source.Close()
	offset, err := localResumeOffset(destination, info.Size())
	if err != nil {
		return err
	}
	progress := &progressWriter{app: a, id: operationID, direction: "download", name: path.Base(remotePath), localPath: destination, total: info.Size(), transferred: offset}
	if offset == info.Size() {
		progress.emit("已完成")
		return nil
	}
	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	target, err := os.OpenFile(destination, flags, 0644)
	if err != nil {
		debugf("download open %s flags=%d: %v", destination, flags, err)
		return fmt.Errorf("无法写入 %s: %w", destination, err)
	}
	defer target.Close()
	if offset > 0 {
		if _, err = source.Seek(offset, io.SeekStart); err != nil {
			debugf("download remote seek %d: %v", offset, err)
			return fmt.Errorf("无法从远程偏移 %d 继续下载: %w", offset, err)
		}
		if _, err = target.Seek(offset, io.SeekStart); err != nil {
			debugf("download local seek %d: %v", offset, err)
			return fmt.Errorf("无法从本地偏移 %d 继续写入: %w", offset, err)
		}
	}
	err = copyWithProgress(target, source, progress, offset)
	if err != nil {
		debugf("download copy %s: %v", destination, err)
	}
	return err
}

func (a *App) UploadFile(id, remoteDirectory, operationID string) error {
	sourcePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择要上传的文件"})
	if err != nil || sourcePath == "" {
		return err
	}
	return a.UploadLocalFile(id, remoteDirectory, sourcePath, operationID)
}

func (a *App) UploadLocalFile(id, remoteDirectory, sourcePath, operationID string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("请拖拽或选择文件，暂不支持上传整个目录")
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	remotePath := path.Join(remoteDirectory, filepath.Base(sourcePath))
	offset := int64(0)
	if remoteInfo, statErr := session.sftp.Stat(remotePath); statErr == nil {
		if remoteInfo.IsDir() {
			return errors.New("远程存在同名目录，无法上传")
		}
		if remoteInfo.Size() <= info.Size() {
			offset = remoteInfo.Size()
		}
	}
	progress := &progressWriter{app: a, id: operationID, direction: "upload", name: filepath.Base(sourcePath), total: info.Size(), transferred: offset}
	if offset == info.Size() {
		progress.emit("已完成")
		return nil
	}
	var target *sftp.File
	if offset > 0 {
		target, err = session.sftp.OpenFile(remotePath, os.O_WRONLY)
		if err != nil {
			return err
		}
		if _, err = target.Seek(offset, io.SeekStart); err != nil {
			target.Close()
			return err
		}
		if _, err = source.Seek(offset, io.SeekStart); err != nil {
			target.Close()
			return err
		}
	} else {
		target, err = session.sftp.Create(remotePath)
		if err != nil {
			return err
		}
	}
	defer target.Close()
	return copyWithProgress(target, source, progress, offset)
}

func localResumeOffset(destination string, remoteSize int64) (int64, error) {
	stat, err := os.Stat(destination)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if stat.IsDir() {
		return 0, errors.New("保存位置是一个目录")
	}
	if stat.Size() > 0 && stat.Size() <= remoteSize {
		return stat.Size(), nil
	}
	return 0, nil
}

func copyWithProgress(dst io.Writer, src io.Reader, progress *progressWriter, offset int64) error {
	status := "进行中"
	if offset > 0 {
		status = "续传中"
	}
	progress.emit(status)
	_, err := io.Copy(dst, io.TeeReader(src, progress))
	if err == nil {
		progress.emit("已完成")
	}
	return err
}

func (a *App) MakeDirectory(id, remoteDirectory, name string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	name = path.Base(strings.TrimSpace(strings.ReplaceAll(name, "\\", "/")))
	if name == "." || name == "" {
		return errors.New("目录名称无效")
	}
	return session.sftp.Mkdir(path.Join(remoteDirectory, name))
}

func (a *App) SetPathMode(id, remotePath, mode string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	mode = strings.TrimSpace(mode)
	if len(mode) == 4 && mode[0] == '0' {
		mode = mode[1:]
	}
	if len(mode) != 3 {
		return errors.New("权限格式无效，请输入 755 或 0644")
	}
	value, err := strconv.ParseUint(mode, 8, 32)
	if err != nil || value > 0777 {
		return errors.New("权限格式无效，只能使用 0 到 7")
	}
	return session.sftp.Chmod(remotePath, os.FileMode(value))
}

func (a *App) SetPathOwner(id, remotePath, owner, group string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	uid, err := strconv.ParseUint(strings.TrimSpace(owner), 10, 32)
	if err != nil {
		return errors.New("所有者 UID 无效")
	}
	gid, err := strconv.ParseUint(strings.TrimSpace(group), 10, 32)
	if err != nil {
		return errors.New("用户组 GID 无效")
	}
	return session.sftp.Chown(remotePath, int(uid), int(gid))
}

func (a *App) OwnershipOptions(id string) (OwnershipOptions, error) {
	session, err := a.getSession(id)
	if err != nil {
		return OwnershipOptions{}, err
	}
	sshSession, err := session.client.NewSession()
	if err != nil {
		return OwnershipOptions{}, err
	}
	defer sshSession.Close()
	output, err := sshSession.CombinedOutput(`awk -F: '{print "u|"$3"|"$1}' /etc/passwd 2>/dev/null; awk -F: '{print "g|"$3"|"$1}' /etc/group 2>/dev/null`)
	if err != nil {
		return OwnershipOptions{}, fmt.Errorf("读取用户和用户组失败: %w", err)
	}
	options := OwnershipOptions{Users: []IdentityOption{}, Groups: []IdentityOption{}}
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(parts) != 3 {
			continue
		}
		identity, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil {
			continue
		}
		option := IdentityOption{ID: identity, Name: parts[2]}
		if parts[0] == "u" {
			options.Users = append(options.Users, option)
		} else if parts[0] == "g" {
			options.Groups = append(options.Groups, option)
		}
	}
	return options, nil
}

func (a *App) DeletePath(id, remotePath string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	info, err := session.sftp.Lstat(remotePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return session.sftp.Remove(remotePath)
	}
	var paths []string
	walker := session.sftp.Walk(remotePath)
	for walker.Step() {
		if walker.Err() != nil {
			return walker.Err()
		}
		paths = append(paths, walker.Path())
	}
	for i := len(paths) - 1; i >= 0; i-- {
		info, statErr := session.sftp.Lstat(paths[i])
		if statErr != nil {
			return statErr
		}
		if info.IsDir() {
			if err = session.sftp.RemoveDirectory(paths[i]); err != nil {
				return err
			}
		} else if err = session.sftp.Remove(paths[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) DownloadDirectory(id, remotePath, operationID string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	destination, err := chooseSavePath("保存目录压缩包", path.Base(strings.TrimRight(remotePath, "/"))+".zip")
	if err != nil || destination == "" {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	root := path.Base(strings.TrimRight(remotePath, "/"))
	var total int64
	sizeWalker := session.sftp.Walk(remotePath)
	for sizeWalker.Step() {
		if info := sizeWalker.Stat(); info != nil && !info.IsDir() {
			total += info.Size()
		}
	}
	progress := &progressWriter{app: a, id: operationID, direction: "download", name: root + ".zip", localPath: destination, total: total}
	progress.emit("进行中")
	walker := session.sftp.Walk(remotePath)
	for walker.Step() {
		if walker.Err() != nil {
			archive.Close()
			output.Close()
			return walker.Err()
		}
		info := walker.Stat()
		if info == nil || info.IsDir() {
			continue
		}
		relative := strings.TrimPrefix(walker.Path(), strings.TrimRight(remotePath, "/")+"/")
		header, headerErr := zip.FileInfoHeader(info)
		if headerErr != nil {
			continue
		}
		header.Name = path.Join(root, relative)
		header.Method = zip.Deflate
		writer, createErr := archive.CreateHeader(header)
		if createErr != nil {
			archive.Close()
			output.Close()
			return createErr
		}
		source, openErr := session.sftp.Open(walker.Path())
		if openErr != nil {
			archive.Close()
			output.Close()
			return openErr
		}
		_, copyErr := io.Copy(writer, io.TeeReader(source, progress))
		source.Close()
		if copyErr != nil {
			archive.Close()
			output.Close()
			return copyErr
		}
	}
	if err = archive.Close(); err != nil {
		output.Close()
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	progress.emit("已完成")
	return nil
}

func (a *App) ShowInFolder(localPath string) error {
	localPath = filepath.Clean(strings.TrimSpace(localPath))
	if localPath == "." || localPath == "" {
		return errors.New("本地文件路径无效")
	}
	if _, err := os.Stat(localPath); err != nil {
		return err
	}
	return exec.Command("explorer.exe", "/select,"+localPath).Start()
}

func (a *App) ScanHostKey(host string, port int) (string, error) {
	if port == 0 {
		port = 22
	}
	address := net.JoinHostPort(strings.TrimSpace(host), fmt.Sprint(port))
	connection, err := net.DialTimeout("tcp", address, 8*time.Second)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	var captured ssh.PublicKey
	config := &ssh.ClientConfig{User: "nekssh-key-scan", HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		captured = key
		return errors.New("host key captured")
	}, Timeout: 8 * time.Second}
	_, _, _, _ = ssh.NewClientConn(connection, address, config)
	if captured == nil {
		return "", errors.New("无法读取 SSH 主机指纹")
	}
	a.mu.Lock()
	a.pendingKeys[address] = captured
	a.mu.Unlock()
	return ssh.FingerprintSHA256(captured), nil
}

func (a *App) ScanConnectionHostKey(req ConnectionRequest) (string, error) {
	if req.Port == 0 {
		req.Port = 22
	}
	address := net.JoinHostPort(strings.TrimSpace(req.Host), strconv.Itoa(req.Port))
	connection, err := dialConnection(req, address)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	var captured ssh.PublicKey
	config := &ssh.ClientConfig{User: "nekssh-key-scan", HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		captured = key
		return errors.New("host key captured")
	}, Timeout: 8 * time.Second}
	_, _, _, _ = ssh.NewClientConn(connection, address, config)
	if captured == nil {
		return "", errors.New("无法读取 SSH 主机指纹")
	}
	a.mu.Lock()
	a.pendingKeys[address] = captured
	a.mu.Unlock()
	return ssh.FingerprintSHA256(captured), nil
}

func (a *App) TrustHostKey(host string, port int) error {
	if port == 0 {
		port = 22
	}
	address := net.JoinHostPort(strings.TrimSpace(host), fmt.Sprint(port))
	a.mu.Lock()
	key := a.pendingKeys[address]
	delete(a.pendingKeys, address)
	a.mu.Unlock()
	if key == nil {
		return errors.New("没有待确认的主机指纹")
	}
	file, err := desktopKnownHostsPath()
	if err != nil {
		return err
	}
	handle, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = fmt.Fprintln(handle, knownhosts.Line([]string{address}, key))
	return err
}

func (a *App) getSession(id string) (*desktopSession, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	session := a.sessions[id]
	if session == nil {
		return nil, errors.New("SSH 会话不存在或已断开")
	}
	return session, nil
}
func (s *desktopSession) stop() {
	s.close.Do(func() {
		close(s.done)
		if s.app != nil {
			s.app.releaseForwards(s)
		}
		if s.sftp != nil {
			s.sftp.Close()
		}
		if s.session != nil {
			s.session.Close()
		}
		if s.client != nil {
			s.client.Close()
		}
	})
}

func desktopAuth(req ConnectionRequest) ([]ssh.AuthMethod, error) {
	var auth []ssh.AuthMethod
	if req.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if req.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(req.PrivateKey), []byte(req.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(req.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("私钥无效: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if req.Password != "" {
		auth = append(auth, ssh.Password(req.Password))
	}
	if len(auth) == 0 {
		return nil, errors.New("请提供密码或私钥")
	}
	return auth, nil
}
func desktopKnownHostsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "NekSSH")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	file := filepath.Join(dir, "known_hosts")
	if _, err = os.Stat(file); errors.Is(err, os.ErrNotExist) {
		if err = os.WriteFile(file, nil, 0600); err != nil {
			return "", err
		}
	}
	return file, nil
}
func desktopHostKeyCallback() (ssh.HostKeyCallback, error) {
	file, err := desktopKnownHostsPath()
	if err != nil {
		return nil, err
	}
	return knownhosts.New(file)
}
func friendlyDesktopError(err error) error {
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unable to authenticate") {
		return errors.New("用户名、密码或私钥不正确")
	}
	if strings.Contains(text, "knownhosts") {
		if strings.Contains(text, "key mismatch") {
			return errors.New("主机指纹已经变化，为防止中间人攻击已拒绝连接")
		}
		return errors.New("主机指纹尚未信任")
	}
	if strings.Contains(text, "connection refused") {
		return errors.New("目标服务器拒绝连接")
	}
	if strings.Contains(text, "timeout") {
		return errors.New("连接超时")
	}
	return err
}

var (
	comdlg32             = windows.NewLazySystemDLL("comdlg32.dll")
	procGetSaveFileName  = comdlg32.NewProc("GetSaveFileNameW")
	procCommDlgExtError  = comdlg32.NewProc("CommDlgExtendedError")
	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWnd = user32.NewProc("GetForegroundWindow")
)

type saveFileNameDialog struct {
	StructSize      uint32
	Owner           uintptr
	Instance        uintptr
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	Hook            uintptr
	TemplateName    *uint16
	Reserved        uintptr
	Reserved2       uint32
	FlagsEx         uint32
}

func chooseSavePath(title, filename string) (string, error) {
	filename = path.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "/" || filename == "" {
		filename = "download"
	}
	buffer := make([]uint16, 32768)
	name, err := windows.UTF16FromString(filename)
	if err != nil {
		return "", err
	}
	copy(buffer, name)
	filter, err := utf16DoubleString("所有文件 (*.*)", "*.*")
	if err != nil {
		return "", err
	}
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	initial := ""
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		downloads := filepath.Join(home, "Downloads")
		if stat, statErr := os.Stat(downloads); statErr == nil && stat.IsDir() {
			initial = downloads
		} else {
			initial = home
		}
	}
	var initialPtr *uint16
	if initial != "" {
		initialPtr, err = windows.UTF16PtrFromString(initial)
		if err != nil {
			return "", err
		}
	}
	owner, _, _ := procGetForegroundWnd.Call()
	dialog := saveFileNameDialog{
		Owner:       owner,
		Filter:      filter,
		FilterIndex: 1,
		File:        &buffer[0],
		MaxFile:     uint32(len(buffer)),
		InitialDir:  initialPtr,
		Title:       titlePtr,
		Flags:       0x00080000 | 0x00000800 | 0x00000002 | 0x00000008,
	}
	dialog.StructSize = uint32(unsafe.Sizeof(dialog))
	ok, _, _ := procGetSaveFileName.Call(uintptr(unsafe.Pointer(&dialog)))
	if ok == 0 {
		code, _, _ := procCommDlgExtError.Call()
		if code == 0 {
			return "", nil
		}
		return "", fmt.Errorf("无法打开保存对话框 (%d)", code)
	}
	return windows.UTF16ToString(buffer), nil
}

func utf16DoubleString(parts ...string) (*uint16, error) {
	encoded := make([]uint16, 0, 64)
	for _, part := range parts {
		item, err := windows.UTF16FromString(part)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, item...)
	}
	encoded = append(encoded, 0)
	return &encoded[0], nil
}

var (
	debugMu   sync.Mutex
	debugFile *os.File
)

func debugLogPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "debug.log"), nil
}

func (a *App) SetDebug(enabled bool) (string, error) {
	logPath, err := debugLogPath()
	if err != nil {
		return "", err
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	if debugFile != nil {
		debugFile.Close()
		debugFile = nil
	}
	if !enabled {
		return logPath, nil
	}
	if err = os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return logPath, err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return logPath, err
	}
	debugFile = file
	fmt.Fprintf(debugFile, "---- debug on %s ----\n", time.Now().Format(time.RFC3339))
	return logPath, nil
}

func debugf(format string, args ...any) {
	debugMu.Lock()
	defer debugMu.Unlock()
	if debugFile == nil {
		return
	}
	fmt.Fprintf(debugFile, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
