package main

import (
	"encoding/base64"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const neksshServerControlPort = 18766

func (a *App) startServerControl() {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(neksshServerControlPort))
	if err != nil {
		debugf("服务器控制口 %d 启动失败: %v", neksshServerControlPort, err)
		return
	}
	server := &http.Server{Handler: a.serverControlMux(), ReadHeaderTimeout: 30 * time.Second}
	go server.Serve(listener)
	debugf("服务器控制口已监听 127.0.0.1:%d", neksshServerControlPort)
}

func (a *App) serverControlMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { a.serverControlIndex(w, r) })
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(serverControlDocumentation))
	})
	mux.HandleFunc("POST /v1/run", func(w http.ResponseWriter, r *http.Request) { a.serverControlRun(w, r) })
	mux.HandleFunc("POST /v1/files", func(w http.ResponseWriter, r *http.Request) { a.serverControlFiles(w, r) })
	mux.HandleFunc("POST /v1/forward", func(w http.ResponseWriter, r *http.Request) { a.serverControlForward(w, r) })
	return mux
}

func (a *App) serverControlIndex(w http.ResponseWriter, r *http.Request) {
	_, err := a.controlSession(r)
	writeReflectJSON(w, http.StatusOK, map[string]any{
		"service":   "nekssh-server",
		"ok":        err == nil,
		"port":      neksshServerControlPort,
		"connected": err == nil,
		"docs":      "GET /docs",
		"usage": map[string]string{
			"run":     `POST /v1/run {"command":"uname -a"}`,
			"files":   `POST /v1/files {"files":[{"path":"/tmp/a.txt","content":"hi"}]}`,
			"forward": `POST /v1/forward {"port":3000}`,
		},
	})
}

func (a *App) serverControlRun(w http.ResponseWriter, r *http.Request) {
	session, err := a.controlSession(r)
	if err != nil {
		writeReflectJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var body struct {
		Command string `json:"command"`
	}
	if err = decodeReflectBody(w, r, &body); err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	command := strings.TrimSpace(body.Command)
	if command == "" {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "缺少 command"})
		return
	}
	output, runErr := runOnServer(session, command)
	result := map[string]any{"ok": runErr == nil, "output": output}
	if runErr != nil {
		result["error"] = runErr.Error()
	}
	writeReflectJSON(w, http.StatusOK, result)
}

func (a *App) serverControlFiles(w http.ResponseWriter, r *http.Request) {
	session, err := a.controlSession(r)
	if err != nil {
		writeReflectJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if session.sftp == nil {
		writeReflectJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "服务器不支持 SFTP"})
		return
	}
	var body struct {
		Path          string        `json:"path"`
		Content       string        `json:"content"`
		ContentBase64 string        `json:"contentBase64"`
		Files         []reflectFile `json:"files"`
	}
	if err = decodeReflectBody(w, r, &body); err != nil {
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
		target, pathErr := remoteControlPath(file.Path)
		if pathErr != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": pathErr.Error()})
			return
		}
		data := []byte(file.Content)
		if file.ContentBase64 != "" {
			data, err = decodeBase64(file.ContentBase64)
			if err != nil {
				writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "contentBase64 无效"})
				return
			}
		}
		if err = session.sftp.MkdirAll(path.Dir(target)); err != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		remote, createErr := session.sftp.Create(target)
		if createErr != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": createErr.Error()})
			return
		}
		_, err = remote.Write(data)
		remote.Close()
		if err != nil {
			writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		written = append(written, target)
	}
	writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "files": written})
}

func (a *App) serverControlForward(w http.ResponseWriter, r *http.Request) {
	session, err := a.controlSession(r)
	if err != nil {
		writeReflectJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var body struct {
		Port int `json:"port"`
	}
	if err = decodeReflectBody(w, r, &body); err != nil {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if body.Port <= 0 || body.Port > 65535 {
		writeReflectJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "端口无效"})
		return
	}
	addr, err := forwardServerPort(session, body.Port)
	if err != nil {
		writeReflectJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeReflectJSON(w, http.StatusOK, map[string]any{"ok": true, "local": addr, "remote": "127.0.0.1:" + strconv.Itoa(body.Port)})
}

func (a *App) controlSession(r *http.Request) (*desktopSession, error) {
	id := strings.TrimSpace(r.URL.Query().Get("session"))
	a.mu.RLock()
	defer a.mu.RUnlock()
	if id != "" {
		session := a.sessions[id]
		if session == nil || session.client == nil {
			return nil, errNoControlSession
		}
		return session, nil
	}
	if len(a.sessions) == 1 {
		for _, session := range a.sessions {
			if session.client == nil {
				return nil, errNoControlSession
			}
			return session, nil
		}
	}
	if len(a.sessions) == 0 {
		return nil, errNoControlSession
	}
	return nil, errManyControlSessions
}

var (
	errNoControlSession   = errString("没有已连接的 SSH 会话")
	errManyControlSessions = errString("有多个会话，请加 ?session=会话ID")
)

type errString string

func (e errString) Error() string { return string(e) }

func runOnServer(session *desktopSession, command string) (string, error) {
	shell, err := session.client.NewSession()
	if err != nil {
		return "", err
	}
	defer shell.Close()
	log := &limitedLog{}
	shell.Stdout = log
	shell.Stderr = log
	err = shell.Run(command)
	return log.String(), err
}

func forwardServerPort(session *desktopSession, port int) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return "", err
	}
	session.addLocalForward(listener)
	target := "127.0.0.1:" + strconv.Itoa(port)
	go func() {
		for {
			local, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			remote, dialErr := session.client.Dial("tcp", target)
			if dialErr != nil {
				local.Close()
				continue
			}
			go bridgeConnections(local, remote)
		}
	}()
	return listener.Addr().String(), nil
}

func remoteControlPath(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.Contains(name, "..") {
		return "", errString("文件路径无效")
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return path.Clean(name), nil
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func (s *desktopSession) addLocalForward(listener net.Listener) {
	s.reflectMu.Lock()
	s.localForwards = append(s.localForwards, listener)
	s.reflectMu.Unlock()
}

func (s *desktopSession) closeLocalForwards() {
	s.reflectMu.Lock()
	listeners := s.localForwards
	s.localForwards = nil
	s.reflectMu.Unlock()
	for _, listener := range listeners {
		listener.Close()
	}
}

const serverControlDocumentation = `NekSSH 服务器控制口
==================

这个地址在客户机的 127.0.0.1:18766。它操作的是当前连上的 SSH 服务器，不是客户机自己。
只有一个会话时自动使用该会话。多个会话时在地址后加 ?session=会话ID。

在服务器上执行命令
------------------

curl -fsS http://127.0.0.1:18766/v1/run \
  -H "Content-Type: application/json" \
  -d "{\"command\":\"uname -a\"}"

返回 ok、output。命令非 0 退出时 ok 为 false，output 里仍有输出。

写服务器文件
------------

路径必须是服务器上的绝对路径，不能包含 .. 。需要服务器开启 SFTP。

curl -fsS http://127.0.0.1:18766/v1/files \
  -H "Content-Type: application/json" \
  -d "{\"files\":[{\"path\":\"/tmp/hello.txt\",\"content\":\"hi\"}]}"

把服务器端口接到客户机
----------------------

把服务器 127.0.0.1 上的端口，接到客户机的同一个端口。

curl -fsS http://127.0.0.1:18766/v1/forward \
  -H "Content-Type: application/json" \
  -d "{\"port\":3000}"

之后客户机访问 http://127.0.0.1:3000/ 会到达服务器的 127.0.0.1:3000。
断开 SSH 后这条转发会关闭。

和 18765 的区别
---------------

18765 反射在服务器上，用来控制客户机。
18766 开在客户机上，用来控制服务器。
`
