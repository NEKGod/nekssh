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
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) { reflectDocs(w, r) })
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
		"docs": "GET /docs",
		"usage": map[string]string{
			"writeFiles": `POST /v1/files {"files":[{"path":"app.py","content":"print(1)"}]}`,
			"run":        `POST /v1/run {"command":"python app.py","port":3000}`,
			"log":        "GET /v1/jobs/{id}/log",
			"stop":       "POST /v1/jobs/{id}/stop",
			"jobs":       "GET /v1/jobs",
		},
	})
}

func reflectDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, reflectDocumentation)
}

const reflectDocumentation = `NekSSH 客户机控制口
==================

这个地址在 SSH 服务器的 127.0.0.1:18765。请求会回到正在连接的 NekSSH 客户机。
客户机是 Windows。命令由 cmd.exe /c 执行。文件写在客户机 NekSSH.exe 旁边的 workspace 目录。

先确认在线
----------

curl -fsS http://127.0.0.1:18765/

返回 JSON 里 service 为 nekssh 且 ok 为 true 才能继续。连不上就让用户先在 NekSSH 里连上这台服务器。不要改在服务器上把服务启动起来充数。

写文件
------

路径相对于 workspace，用 / 分隔，不能包含 .. 。
文本放 content。二进制放 contentBase64，不要同时依赖 content。

curl -fsS http://127.0.0.1:18765/v1/files \
  -H "Content-Type: application/json" \
  -d "{\"files\":[{\"path\":\"server.py\",\"content\":\"print(1)\"}]}"

也接受单个文件：{"path":"server.py","content":"..."}

成功返回 {"ok":true,"files":["server.py"]}。

运行并反射端口
--------------

command 在 workspace 里执行。port 是程序在客户机上监听的端口。
程序应监听 127.0.0.1，并优先读取环境变量 PORT。
反射成功后，服务器上用同一个端口访问，地址形如 http://127.0.0.1:3000/ 。

curl -fsS http://127.0.0.1:18765/v1/run \
  -H "Content-Type: application/json" \
  -d "{\"command\":\"python -u server.py\",\"port\":3000}"

成功时 ok 为 true，并包含：
- id：任务编号，例如 job-1
- pid：客户机进程号
- local：客户机地址
- remote：服务器上的访问地址

存在 reflectError 就是反射失败。先看日志，再停止任务，不要告诉用户已经可以访问。
不需要反射端口时省略 port，只在客户机执行命令。

日志、状态、停止
----------------

curl -fsS http://127.0.0.1:18765/v1/jobs
curl -fsS http://127.0.0.1:18765/v1/jobs/job-1
curl -fsS http://127.0.0.1:18765/v1/jobs/job-1/log
curl -fsS -X POST http://127.0.0.1:18765/v1/jobs/job-1/stop

停止会结束该任务在客户机上的整个进程树。

约束
----

- 只使用 127.0.0.1:18765 这个控制口，不要扫描别的端口。
- 18765 是控制口，不要拿来跑你的业务服务。
- 业务端口同一时间只反射一次。被占用就换一个大于 1024 的端口再运行。
- 文件只写进 workspace。命令只做用户要求的那件事。
- remote 地址只在这台服务器本机可用，不要把它写成公网地址。

客户机上的 127.0.0.1:18766 用来反过来控制这台服务器：执行命令、写文件、把服务器端口接到客户机。说明见客户机的 http://127.0.0.1:18766/docs 。
`

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
