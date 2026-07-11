package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type App struct {
	ctx         context.Context
	mu          sync.RWMutex
	sessions    map[string]*desktopSession
	pendingKeys map[string]ssh.PublicKey
}

type desktopSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	sftp    *sftp.Client
	close   sync.Once
	notify  sync.Once
	done    chan struct{}
}

type ConnectionRequest struct {
	ID, Host, Username, Password, PrivateKey, Passphrase string
	Port, Cols, Rows                                     int
}

type FileEntry struct {
	Name, Path, Modified, Mode, Owner, Group string
	Size                                     int64
	Directory                                bool
}

func NewApp() *App {
	return &App{sessions: make(map[string]*desktopSession), pendingKeys: make(map[string]ssh.PublicKey)}
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
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
	auth, err := desktopAuth(req)
	if err != nil {
		return err
	}
	callback, err := desktopHostKeyCallback()
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(req.Host, fmt.Sprint(req.Port)), &ssh.ClientConfig{User: req.Username, Auth: auth, HostKeyCallback: callback, Timeout: 10 * time.Second})
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
	entry := &desktopSession{client: client, session: shell, stdin: stdin, sftp: sftpClient, done: make(chan struct{})}
	a.mu.Lock()
	if old := a.sessions[req.ID]; old != nil {
		old.stop()
	}
	a.sessions[req.ID] = entry
	a.mu.Unlock()
	go a.stream(req.ID, entry, stdout)
	go a.stream(req.ID, entry, stderr)
	go a.keepalive(req.ID, entry)
	runtime.EventsEmit(a.ctx, "ssh:connected", req.ID)
	return nil
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

func (a *App) DownloadFile(id, remotePath string) error {
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
	destination, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "保存远程文件", DefaultFilename: path.Base(remotePath)})
	if err != nil || destination == "" {
		return err
	}
	source, err := session.sftp.Open(remotePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

func (a *App) UploadFile(id, remoteDirectory string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	if session.sftp == nil {
		return errors.New("目标服务器不支持 SFTP")
	}
	sourcePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择要上传的文件"})
	if err != nil || sourcePath == "" {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := session.sftp.Create(path.Join(remoteDirectory, filepath.Base(sourcePath)))
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
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

func (a *App) DownloadDirectory(id, remotePath string) error {
	session, err := a.getSession(id)
	if err != nil {
		return err
	}
	destination, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "保存目录压缩包", DefaultFilename: path.Base(strings.TrimRight(remotePath, "/")) + ".zip"})
	if err != nil || destination == "" {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	root := path.Base(strings.TrimRight(remotePath, "/"))
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
		_, copyErr := io.Copy(writer, source)
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
	return output.Close()
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
