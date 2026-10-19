package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const neksshReflectPort = 18765

var reflectWorkspaceDir string

type clientJob struct {
	ID, Command, Remote string
	Port, PID           int
	cmd                 *exec.Cmd
	log                 *limitedLog
	done                chan struct{}
}

type limitedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *limitedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buf.Len() > 256*1024 {
		return len(p), nil
	}
	space := 256*1024 - l.buf.Len()
	if len(p) > space {
		p = p[:space]
	}
	return l.buf.Write(p)
}

func (l *limitedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (a *App) startReflectServer() {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(neksshReflectPort))
	if err != nil {
		debugf("本机反射端口 %d 启动失败: %v", neksshReflectPort, err)
		return
	}
	server := &http.Server{Handler: a.reflectMux(""), ReadHeaderTimeout: 30 * time.Second}
	a.reflectServer = server
	go server.Serve(listener)
	debugf("本机反射端口已监听 127.0.0.1:%d", neksshReflectPort)
}

func (a *App) serveReflect(id string, session *desktopSession) {
	listener, err := session.client.Listen("tcp", "127.0.0.1:"+strconv.Itoa(neksshReflectPort))
	if err != nil {
		debugf("反射到服务器失败: %v", err)
		a.emitReflect("reflect:notice", id, "本机端口未能反射到服务器："+err.Error())
		return
	}
	session.setReflectListener(listener)
	remoteAddr := listener.Addr().String()
	debugf("已反射到服务器 %s", remoteAddr)
	a.emitReflect("reflect:ready", id, remoteAddr)
	server := &http.Server{Handler: a.reflectMux(id), ReadHeaderTimeout: 30 * time.Second}
	server.Serve(listener)
}

func (a *App) reflectMux(sessionID string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { a.reflectIndex(w, r) })
	mux.HandleFunc("POST /v1/files", func(w http.ResponseWriter, r *http.Request) { a.reflectWriteFiles(w, r) })
	mux.HandleFunc("POST /v1/run", func(w http.ResponseWriter, r *http.Request) { a.reflectRun(sessionID, w, r) })
	mux.HandleFunc("GET /v1/jobs", func(w http.ResponseWriter, r *http.Request) { a.reflectJobs(w, r) })
	mux.HandleFunc("GET /v1/jobs/", func(w http.ResponseWriter, r *http.Request) { a.reflectJobRoute(w, r) })
	mux.HandleFunc("POST /v1/jobs/", func(w http.ResponseWriter, r *http.Request) { a.reflectJobRoute(w, r) })
	return mux
}

func (a *App) reflectIndex(w http.ResponseWriter, r *http.Request) {
	workspace, _ := reflectWorkspace()
	writeReflectJSON(w, http.StatusOK, map[string]any{
		"service":   "nekssh",
		"ok":        true,
		"port":      neksshReflectPort,
		"workspace": workspace,
		"usage": map[string]string{
			"writeFiles": `POST /v1/files {"files":[{"path":"app.py","content":"print(1)"}]}`,
			"run":        `POST /v1/run {"command":"python app.py","port":3000}`,
			"log":        "GET /v1/jobs/{id}/log",
			"stop":       "POST /v1/jobs/{id}/stop",
			"jobs":       "GET /v1/jobs",
		},
	})
}

type reflectFile struct {
	Path          string `json:"path"`
	Content       string `json:"content"`
	ContentBase64 string `json:"contentBase64"`
}

func (a *App) reflectWriteFiles(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path          string        `json:"path"`
		Content       string        `json:"content"`
		ContentBase64 string        `json:"contentBase64"`
		Files         []reflectFile `json:"files"`
	}
	if err := decodeReflectBody(w, r, &body); err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	files := body.Files
	if body.Path != "" {
		files = append(files, reflectFile{Path: body.Path, Content: body.Content, ContentBase64: body.ContentBase64})
	}
	if len(files) == 0 {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "没有要写入的文件"})
		return
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		target, err := reflectFilePath(file.Path)
		if err != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		data := []byte(file.Content)
		if file.ContentBase64 != "" {
			data, err = base64.StdEncoding.DecodeString(file.ContentBase64)
			if err != nil {
				writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "contentBase64 无效"})
				return
			}
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err = os.WriteFile(target, data, 0600); err != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		written = append(written, file.Path)
	}
	writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "files": written})
}

func (a *App) reflectRun(sessionID string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
		Port    int    `json:"port"`
	}
	if err := decodeReflectBody(w, r, &body); err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	command := strings.TrimSpace(body.Command)
	if command == "" {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "缺少 command"})
		return
	}
	if body.Port < 0 || body.Port > 65535 {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "端口无效"})
		return
	}
	workspace, err := reflectWorkspace()
	if err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	job := &clientJob{Command: command, Port: body.Port, log: &limitedLog{}, done: make(chan struct{})}
	a.jobMu.Lock()
	a.jobSeq++
	job.ID = fmt.Sprintf("job-%d", a.jobSeq)
	if a.jobs == nil {
		a.jobs = map[string]*clientJob{}
	}
	a.jobs[job.ID] = job
	a.jobMu.Unlock()
	cmd := exec.Command("cmd.exe", "/c", command)
	cmd.Dir = workspace
	cmd.Stdout = job.log
	cmd.Stderr = job.log
	if body.Port > 0 {
		cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(body.Port))
	}
	if err = cmd.Start(); err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	job.cmd = cmd
	job.PID = cmd.Process.Pid
	go func() {
		_ = cmd.Wait()
		close(job.done)
	}()
	result := map[string]any{"ok": true, "id": job.ID, "pid": job.PID, "command": command, "workspace": workspace}
	if body.Port > 0 {
		addrs, reflectErr := a.reflectLocalPort(sessionID, body.Port)
		result["port"] = body.Port
		result["local"] = "127.0.0.1:" + strconv.Itoa(body.Port)
		if reflectErr != nil {
			result["reflectError"] = reflectErr.Error()
		} else {
			result["remote"] = addrs
			job.Remote = strings.Join(addrs, ",")
		}
	}
	writeReflectJSON(w, http.StatusOK, result)
}

func (a *App) reflectJobs(w http.ResponseWriter, r *http.Request) {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()
	list := make([]map[string]any, 0, len(a.jobs))
	for _, job := range a.jobs {
		list = append(list, job.snapshot())
	}
	writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "jobs": list})
}

func (a *App) reflectJobRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	id, action, _ := strings.Cut(rest, "/")
	job := a.jobByID(id)
	if job == nil {
		writeReflectJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "任务不存在"})
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "log":
		writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "id": job.ID, "log": job.log.String()})
	case r.Method == http.MethodPost && action == "stop":
		a.stopJob(job)
		writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "id": job.ID, "stopped": true})
	case r.Method == http.MethodGet && action == "":
		writeReflectJSON(w, http.StatusOK, job.snapshot())
	default:
		writeReflectJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "未知操作"})
	}
}

func (job *clientJob) snapshot() map[string]any {
	running := true
	select {
	case <-job.done:
		running = false
	default:
	}
	return map[string]any{"id": job.ID, "command": job.Command, "port": job.Port, "pid": job.PID, "remote": job.Remote, "running": running}
}

func (a *App) jobByID(id string) *clientJob {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()
	return a.jobs[id]
}

func (a *App) stopJob(job *clientJob) {
	if job == nil || job.cmd == nil || job.cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(job.cmd.Process.Pid)).Run()
	_ = job.cmd.Process.Kill()
}

func (a *App) stopAllJobs() {
	a.jobMu.Lock()
	jobs := make([]*clientJob, 0, len(a.jobs))
	for _, job := range a.jobs {
		jobs = append(jobs, job)
	}
	a.jobMu.Unlock()
	for _, job := range jobs {
		a.stopJob(job)
	}
}

func (a *App) reflectLocalPort(sessionID string, port int) ([]string, error) {
	a.mu.RLock()
	targets := make([]*desktopSession, 0, 1)
	if sessionID != "" {
		if session := a.sessions[sessionID]; session != nil {
			targets = append(targets, session)
		}
	} else {
		for _, session := range a.sessions {
			targets = append(targets, session)
		}
	}
	a.mu.RUnlock()
	if len(targets) == 0 {
		return nil, errors.New("没有已连接的 SSH 会话，端口未反射")
	}
	addrs := make([]string, 0, len(targets))
	var firstErr error
	for _, session := range targets {
		addr, err := forwardRemotePort(session, port)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return nil, firstErr
	}
	return addrs, nil
}

func forwardRemotePort(session *desktopSession, port int) (string, error) {
	listener, err := session.client.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return "", err
	}
	session.addReflectListener(listener)
	go func() {
		for {
			remote, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			local, dialErr := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
			if dialErr != nil {
				remote.Close()
				continue
			}
			go bridgeConnections(remote, local)
		}
	}()
	return listener.Addr().String(), nil
}

func reflectWorkspace() (string, error) {
	if reflectWorkspaceDir != "" {
		if err := os.MkdirAll(reflectWorkspaceDir, 0700); err != nil {
			return "", err
		}
		return reflectWorkspaceDir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(exe), "workspace")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func reflectFilePath(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.Contains(name, "..") {
		return "", errors.New("文件路径无效")
	}
	root, err := reflectWorkspace()
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("文件路径无效")
	}
	return target, nil
}

func decodeReflectBody(w http.ResponseWriter, r *http.Request, dest any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(dest)
}

func writeReflectJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func (s *desktopSession) setReflectListener(listener net.Listener) {
	s.reflectMu.Lock()
	s.reflectListener = listener
	s.reflectMu.Unlock()
}

func (s *desktopSession) addReflectListener(listener net.Listener) {
	s.reflectMu.Lock()
	s.reflectMore = append(s.reflectMore, listener)
	s.reflectMu.Unlock()
}

func (s *desktopSession) closeReflect() {
	s.reflectMu.Lock()
	listeners := make([]net.Listener, 0, 1+len(s.reflectMore))
	if s.reflectListener != nil {
		listeners = append(listeners, s.reflectListener)
	}
	listeners = append(listeners, s.reflectMore...)
	s.reflectListener = nil
	s.reflectMore = nil
	s.reflectMu.Unlock()
	for _, listener := range listeners {
		listener.Close()
	}
}

func (a *App) emitReflect(event, id, message string) {
	if a.ctx != nil {
		emitReflectEvent(a, event, id, message)
	}
}
