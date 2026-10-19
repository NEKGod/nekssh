package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestRemoteTools(t *testing.T) {
	client := liveClient(t)
	t.Cleanup(func() { client.Close() })
	fmt.Println(remoteOnce(t, client, "command -v grok; command -v curl; command -v python3; uname -a"))
}

func liveClient(t *testing.T) *ssh.Client {
	t.Helper()
	host := os.Getenv("NEKSSH_TEST_HOST")
	user := os.Getenv("NEKSSH_TEST_USER")
	password := os.Getenv("NEKSSH_TEST_PASSWORD")
	if host == "" || user == "" || password == "" {
		t.Skip("set NEKSSH_TEST_HOST, NEKSSH_TEST_USER and NEKSSH_TEST_PASSWORD")
	}
	port := 8022
	if value := os.Getenv("NEKSSH_TEST_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		port = parsed
	}
	client, err := dialSSH(ConnectionRequest{Host: host, Port: port}, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func remoteOnce(t *testing.T, client *ssh.Client, command string) string {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	output, err := session.CombinedOutput(command)
	text := strings.TrimSpace(string(output))
	if err != nil {
		t.Fatalf("%s\n%s", err, text)
	}
	return text
}

func TestGrokBuildsReflectedService(t *testing.T) {
	client := liveClient(t)
	t.Cleanup(func() { client.Close() })
	reflectWorkspaceDir = t.TempDir()
	app := NewApp()
	session := &desktopSession{client: client, done: make(chan struct{}), app: app}
	app.sessions["live"] = session
	go app.serveReflect("live", session)
	t.Cleanup(session.closeReflect)
	t.Cleanup(app.stopAllJobs)
	remoteCurl(t, client, "curl -fsS --max-time 10 http://127.0.0.1:18765/")

	prompt := `你在 SSH 服务器上。NekSSH 客户机是 Windows，已经把控制口反射到本机 http://127.0.0.1:18765/ 。
请严格按下面的技能做，不要在服务器上启动这个 HTTP 服务。

客户机执行命令的方式是 cmd.exe /c。用 python 启动服务。服务必须监听 127.0.0.1，端口读取环境变量 PORT。
请开发一个最小 HTTP 服务：GET / 返回纯文本 hello-from-nekssh。
使用端口 18772。
步骤：
1. curl http://127.0.0.1:18765/ 确认 ok。
2. POST /v1/files 把服务源码写到客户机，例如 server.py。
3. POST /v1/run ，command 为 python -u server.py ，port 为 18772。
4. 如果返回 reflectError，读取 /v1/jobs/{id}/log 并停止，不要声称成功。
5. 在本服务器上 curl http://127.0.0.1:18772/ ，确认响应正文是 hello-from-nekssh。
6. 最终只输出这个响应正文、任务 id 和 remote 地址。
`
	uploadRemote(t, client, "/tmp/nekssh-task.md", prompt)
	fmt.Println(streamRemote(t, client, "killall grok 2>/dev/null || true; /root/.grok/bin/grok --prompt-file /tmp/nekssh-task.md --always-approve --max-turns 12"))
	body := remoteOnce(t, client, "curl -fsS --max-time 10 http://127.0.0.1:18772/")
	fmt.Println("server response:", body)
	if !strings.Contains(body, "hello-from-nekssh") {
		t.Fatalf("reflected service response %s", body)
	}
}

func streamRemote(t *testing.T, client *ssh.Client, command string) string {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = session.Start(command); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	done := make(chan struct{}, 2)
	copyStream := func(reader io.Reader) {
		buffer := make([]byte, 1024)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				chunk := string(buffer[:n])
				builder.WriteString(chunk)
				fmt.Print(chunk)
			}
			if readErr != nil {
				done <- struct{}{}
				return
			}
		}
	}
	go copyStream(stdout)
	go copyStream(stderr)
	<-done
	<-done
	_ = session.Wait()
	return builder.String()
}

func uploadRemote(t *testing.T, client *ssh.Client, path, content string) {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Stdin = strings.NewReader(content)
	if err = session.Run("cat > " + path); err != nil {
		t.Fatal(err)
	}
}

func TestReflectLiveSSH(t *testing.T) {
	host := os.Getenv("NEKSSH_TEST_HOST")
	user := os.Getenv("NEKSSH_TEST_USER")
	password := os.Getenv("NEKSSH_TEST_PASSWORD")
	if host == "" || user == "" || password == "" {
		t.Skip("set NEKSSH_TEST_HOST, NEKSSH_TEST_USER and NEKSSH_TEST_PASSWORD")
	}
	port := 8022
	if value := os.Getenv("NEKSSH_TEST_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		port = parsed
	}
	reflectWorkspaceDir = t.TempDir()
	client, err := dialSSH(ConnectionRequest{Host: host, Port: port}, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })

	app := NewApp()
	session := &desktopSession{client: client, done: make(chan struct{}), app: app}
	app.sessions["live"] = session
	go app.serveReflect("live", session)
	t.Cleanup(session.closeReflect)
	t.Cleanup(app.stopAllJobs)

	index := remoteCurl(t, client, "curl -fsS --max-time 10 http://127.0.0.1:18765/")
	if !strings.Contains(index, `"service":"nekssh"`) || !strings.Contains(index, `"ok":true`) {
		t.Fatalf("index %s", index)
	}

	wrote := remoteCurl(t, client, `curl -fsS --max-time 10 http://127.0.0.1:18765/v1/files -H "Content-Type: application/json" -d "{\"path\":\"live.txt\",\"content\":\"from-server\"}"`)
	if !strings.Contains(wrote, `"ok":true`) {
		t.Fatalf("write %s", wrote)
	}

	ran := remoteCurl(t, client, `curl -fsS --max-time 15 http://127.0.0.1:18765/v1/run -H "Content-Type: application/json" -d "{\"command\":\"echo nekssh-live\"}"`)
	if !strings.Contains(ran, `"id":"`) {
		t.Fatalf("run %s", ran)
	}

	probe, err := net.Listen("tcp", "127.0.0.1:18771")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { probe.Close() })
	go http.Serve(probe, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "reflected-ok")
	}))
	if _, err = forwardRemotePort(session, 18771); err != nil {
		t.Fatal(err)
	}
	reflected := remoteCurl(t, client, "curl -fsS --max-time 10 http://127.0.0.1:18771/")
	if !strings.Contains(reflected, "reflected-ok") {
		t.Fatalf("reflected port %s", reflected)
	}
}

func remoteCurl(t *testing.T, client *ssh.Client, command string) string {
	t.Helper()
	var last string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		session, err := client.NewSession()
		if err != nil {
			t.Fatal(err)
		}
		output, err := session.CombinedOutput(command)
		session.Close()
		last = strings.TrimSpace(string(output))
		if err == nil && last != "" {
			return last
		}
		if err != nil && last != "" {
			last = last + " (" + err.Error() + ")"
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("remote command failed: %s\n%s", command, last)
	return ""
}
