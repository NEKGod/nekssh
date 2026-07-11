package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

//go:embed web/*
var webFiles embed.FS

type connectRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}

type clientMessage struct {
	Type string          `json:"type"`
	Data string          `json:"data,omitempty"`
	Cols int             `json:"cols,omitempty"`
	Rows int             `json:"rows,omitempty"`
	Conn *connectRequest `json:"connection,omitempty"`
	Path string          `json:"path,omitempty"`
	Name string          `json:"name,omitempty"`
}

type serverMessage struct {
	Type    string      `json:"type"`
	Data    string      `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Code    string      `json:"code,omitempty"`
	Retry   bool        `json:"retry,omitempty"`
	Path    string      `json:"path,omitempty"`
	Name    string      `json:"name,omitempty"`
	Entries []fileEntry `json:"entries,omitempty"`
	Count   int         `json:"count,omitempty"`
	Size    int64       `json:"size,omitempty"`
}

type fileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Directory bool   `json:"directory"`
	Modified  string `json:"modified"`
	Mode      string `json:"mode"`
	Owner     string `json:"owner"`
	Group     string `json:"group"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || strings.Contains(origin, r.Host)
	},
}

func main() {
	addr := env("NEKSSH_ADDR", "127.0.0.1:8022")
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	mux.HandleFunc("/ws", terminalHandler)
	mux.Handle("/", http.FileServer(http.FS(sub)))
	srv := &http.Server{Addr: addr, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("NekSSH running at http://%s", addr)
	log.Fatal(srv.ListenAndServe())
}

func terminalHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket request from %s", r.RemoteAddr)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()
	ws.SetReadLimit(16 << 20)

	var first clientMessage
	if err := ws.ReadJSON(&first); err != nil || first.Type != "connect" || first.Conn == nil {
		writeJSON(ws, serverMessage{Type: "error", Message: "无效的连接请求"})
		return
	}
	cfg := first.Conn
	if err := validate(cfg); err != nil {
		writeJSON(ws, serverMessage{Type: "error", Message: err.Error()})
		return
	}
	log.Printf("SSH connecting: user=%q host=%q port=%d", cfg.Username, cfg.Host, cfg.Port)

	client, err := dialSSH(cfg)
	if err != nil {
		log.Printf("SSH connection failed: user=%q host=%q port=%d error=%v", cfg.Username, cfg.Host, cfg.Port, err)
		writeJSON(ws, classifySSHError(err))
		return
	}
	defer client.Close()
	sftpClient, sftpErr := sftp.NewClient(client)
	if sftpErr == nil {
		defer sftpClient.Close()
	}
	session, err := client.NewSession()
	if err != nil {
		writeJSON(ws, serverMessage{Type: "error", Message: err.Error()})
		return
	}
	defer session.Close()

	cols, rows := clamp(cfg.Cols, 20, 500, 100), clamp(cfg.Rows, 5, 200, 30)
	if err := session.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}); err != nil {
		writeJSON(ws, serverMessage{Type: "error", Message: "无法创建终端: " + err.Error()})
		return
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	if err := session.Shell(); err != nil {
		writeJSON(ws, serverMessage{Type: "error", Message: err.Error()})
		return
	}
	writeJSON(ws, serverMessage{Type: "connected"})

	var writeMu sync.Mutex
	send := func(v serverMessage) error { writeMu.Lock(); defer writeMu.Unlock(); return writeJSON(ws, v) }
	stream := func(rd io.Reader) {
		buf := make([]byte, 8192)
		for {
			n, err := rd.Read(buf)
			if n > 0 {
				if send(serverMessage{Type: "output", Data: string(buf[:n])}) != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}
	go stream(stdout)
	go stream(stderr)
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				_ = send(serverMessage{Type: "closed", Code: "CONNECTION_LOST", Message: "SSH 连接已中断", Retry: true})
				_ = ws.Close()
				return
			}
		}
	}()

	for {
		var msg clientMessage
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}
		switch msg.Type {
		case "input":
			_, err = io.WriteString(stdin, msg.Data)
		case "resize":
			err = session.WindowChange(clamp(msg.Rows, 5, 200, 30), clamp(msg.Cols, 20, 500, 100))
		case "sftp_list":
			err = handleSFTPList(sftpClient, msg.Path, send)
		case "sftp_upload":
			err = handleSFTPUpload(sftpClient, msg.Path, msg.Name, msg.Data, send)
		case "sftp_download":
			err = handleSFTPDownload(sftpClient, msg.Path, send)
		case "sftp_estimate":
			err = handleSFTPEstimate(sftpClient, msg.Path, send)
		case "sftp_download_dir":
			err = handleSFTPDirectoryDownload(sftpClient, msg.Path, send)
		}
		if err != nil {
			break
		}
	}
}

const maxTransferSize = 10 << 20

func handleSFTPList(client *sftp.Client, remotePath string, send func(serverMessage) error) error {
	if client == nil {
		return send(serverMessage{Type: "sftp_error", Message: "目标服务器不支持 SFTP"})
	}
	if remotePath == "" {
		remotePath = "."
	}
	if len(remotePath) > 4096 {
		return send(serverMessage{Type: "sftp_error", Message: "目录路径过长"})
	}
	resolvedPath, err := client.RealPath(remotePath)
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "无法解析目录路径: " + err.Error()})
	}
	items, err := client.ReadDir(resolvedPath)
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "无法读取目录: " + err.Error()})
	}
	entries := make([]fileEntry, 0, len(items))
	for _, item := range items {
		entry := fileEntry{Name: item.Name(), Path: path.Join(resolvedPath, item.Name()), Size: item.Size(), Directory: item.IsDir(), Modified: item.ModTime().Format(time.RFC3339), Mode: item.Mode().String()}
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
	return send(serverMessage{Type: "sftp_list", Path: resolvedPath, Entries: entries})
}

func handleSFTPUpload(client *sftp.Client, remotePath, name, encoded string, send func(serverMessage) error) error {
	if client == nil {
		return send(serverMessage{Type: "sftp_error", Message: "目标服务器不支持 SFTP"})
	}
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "" {
		return send(serverMessage{Type: "sftp_error", Message: "文件名无效"})
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "上传数据无效"})
	}
	if len(data) > maxTransferSize {
		return send(serverMessage{Type: "sftp_error", Message: "单个文件不能超过 10 MB"})
	}
	file, err := client.Create(path.Join(remotePath, name))
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "无法创建远程文件: " + err.Error()})
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		err = writeErr
	} else {
		err = closeErr
	}
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "上传失败: " + err.Error()})
	}
	return send(serverMessage{Type: "sftp_uploaded", Name: name, Message: "上传完成"})
}

func handleSFTPDownload(client *sftp.Client, remotePath string, send func(serverMessage) error) error {
	if client == nil {
		return send(serverMessage{Type: "sftp_error", Message: "目标服务器不支持 SFTP"})
	}
	file, err := client.Open(remotePath)
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "无法打开文件: " + err.Error()})
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: err.Error()})
	}
	if info.Size() > maxTransferSize {
		return send(serverMessage{Type: "sftp_error", Message: "下载文件不能超过 10 MB"})
	}
	data, err := io.ReadAll(io.LimitReader(file, maxTransferSize+1))
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "下载失败: " + err.Error()})
	}
	return send(serverMessage{Type: "sftp_download", Name: path.Base(remotePath), Data: base64.StdEncoding.EncodeToString(data)})
}

const maxDirectorySize = 50 << 20

func estimateDirectory(client *sftp.Client, remotePath string) (int, int64, error) {
	count, size := 0, int64(0)
	walker := client.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return 0, 0, err
		}
		info := walker.Stat()
		if info != nil && !info.IsDir() {
			count++
			size += info.Size()
			if size > maxDirectorySize {
				return count, size, fmt.Errorf("目录内容超过 50 MB 限制")
			}
		}
	}
	return count, size, nil
}

func handleSFTPEstimate(client *sftp.Client, remotePath string, send func(serverMessage) error) error {
	if client == nil {
		return send(serverMessage{Type: "sftp_error", Message: "目标服务器不支持 SFTP"})
	}
	count, size, err := estimateDirectory(client, remotePath)
	if err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "无法统计目录: " + err.Error()})
	}
	return send(serverMessage{Type: "sftp_estimate", Path: remotePath, Count: count, Size: size})
}

func handleSFTPDirectoryDownload(client *sftp.Client, remotePath string, send func(serverMessage) error) error {
	if client == nil {
		return send(serverMessage{Type: "sftp_error", Message: "目标服务器不支持 SFTP"})
	}
	if _, _, err := estimateDirectory(client, remotePath); err != nil {
		return send(serverMessage{Type: "sftp_error", Message: err.Error()})
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	rootName := path.Base(strings.TrimRight(remotePath, "/"))
	walker := client.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			archive.Close()
			return send(serverMessage{Type: "sftp_error", Message: "目录读取失败: " + err.Error()})
		}
		info := walker.Stat()
		if info == nil || info.IsDir() {
			continue
		}
		relative := strings.TrimPrefix(walker.Path(), strings.TrimRight(remotePath, "/")+"/")
		if relative == walker.Path() || relative == "" {
			continue
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			continue
		}
		header.Name = path.Join(rootName, relative)
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			archive.Close()
			return send(serverMessage{Type: "sftp_error", Message: err.Error()})
		}
		file, err := client.Open(walker.Path())
		if err != nil {
			archive.Close()
			return send(serverMessage{Type: "sftp_error", Message: err.Error()})
		}
		_, copyErr := io.Copy(writer, io.LimitReader(file, maxDirectorySize))
		file.Close()
		if copyErr != nil {
			archive.Close()
			return send(serverMessage{Type: "sftp_error", Message: copyErr.Error()})
		}
	}
	if err := archive.Close(); err != nil {
		return send(serverMessage{Type: "sftp_error", Message: "压缩失败: " + err.Error()})
	}
	return send(serverMessage{Type: "sftp_download", Name: rootName + ".zip", Data: base64.StdEncoding.EncodeToString(buffer.Bytes())})
}

func dialSSH(c *connectRequest) (*ssh.Client, error) {
	var auth []ssh.AuthMethod
	if c.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if c.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(c.PrivateKey), []byte(c.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(c.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("私钥无效: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if c.Password != "" {
		auth = append(auth, ssh.Password(c.Password))
	}
	if len(auth) == 0 {
		return nil, errors.New("请提供密码或私钥")
	}
	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{User: c.Username, Auth: auth, HostKeyCallback: hostKeyCallback, Timeout: 10 * time.Second}
	return ssh.Dial("tcp", net.JoinHostPort(c.Host, fmt.Sprint(c.Port)), config)
}

func hostKeyCallback() (ssh.HostKeyCallback, error) {
	path := os.Getenv("NEKSSH_KNOWN_HOSTS")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	if _, err := os.Stat(path); err == nil {
		return knownhosts.New(path)
	}
	if os.Getenv("NEKSSH_INSECURE_HOST_KEY") == "1" {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return nil, fmt.Errorf("未找到 known_hosts (%s)；请先用系统 ssh 接受主机指纹，或仅在开发环境设置 NEKSSH_INSECURE_HOST_KEY=1", path)
}

func validate(c *connectRequest) error {
	c.Host, c.Username = strings.TrimSpace(c.Host), strings.TrimSpace(c.Username)
	if c.Host == "" || c.Username == "" {
		return errors.New("主机和用户名不能为空")
	}
	if strings.ContainsAny(c.Host, "\r\n\x00") {
		return errors.New("主机地址无效")
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("端口无效")
	}
	return nil
}

func friendlySSHError(err error) string {
	return classifySSHError(err).Message
}

func classifySSHError(err error) serverMessage {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unable to authenticate"), strings.Contains(message, "no supported methods remain"):
		return serverMessage{Type: "error", Code: "AUTH_FAILED", Message: "用户名、密码或私钥不正确，请检查后重试", Retry: true}
	case strings.Contains(message, "connection refused"):
		return serverMessage{Type: "error", Code: "CONNECTION_REFUSED", Message: "目标服务器拒绝连接，请检查 SSH 端口和服务状态", Retry: true}
	case strings.Contains(message, "i/o timeout"), strings.Contains(message, "deadline exceeded"):
		return serverMessage{Type: "error", Code: "TIMEOUT", Message: "连接超时，请检查主机地址、端口和网络", Retry: true}
	case strings.Contains(message, "no such host"):
		return serverMessage{Type: "error", Code: "HOST_NOT_FOUND", Message: "找不到目标主机，请检查主机地址", Retry: true}
	case strings.Contains(message, "knownhosts"), strings.Contains(message, "host key"):
		return serverMessage{Type: "error", Code: "HOST_KEY_FAILED", Message: "主机指纹校验失败，请先确认服务器 SSH 指纹"}
	default:
		return serverMessage{Type: "error", Code: "SSH_ERROR", Message: "SSH 连接失败: " + err.Error(), Retry: true}
	}
}

func writeJSON(ws *websocket.Conn, v serverMessage) error {
	ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ws.WriteJSON(v)
}
func clamp(v, lo, hi, fallback int) int {
	if v == 0 {
		return fallback
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-store, max-age=0")
		}
		next.ServeHTTP(w, r)
	})
}
