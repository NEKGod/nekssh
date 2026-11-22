package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const agentSkillDir = ".grok/skills/run-on-nekssh-client"
const agentSkillFile = "SKILL.md"
const agentClientsDir = ".nekssh/clients"

const agentSkillMarkdown = `---
name: run-on-nekssh-client
description: 把当前 SSH 服务器上的代码放到 NekSSH 客户机运行，并把服务端口反射回这台服务器。用户说在客户机、本机、NekSSH 上运行，或要把端口反射给这边用时使用。
when-to-use: 在 NekSSH 客户机上运行、跑到客户机、反射端口、让我电脑跑这个服务、run on the NekSSH client
---

# 在 NekSSH 客户机上运行并反射端口

NekSSH 桌面端连着这台 SSH 服务器时，会在服务器本机开放一个控制口。你通过它把文件写到用户电脑、在用户电脑上启动进程，并把该进程的端口反射回这台服务器的 127.0.0.1。

控制口由 SSH 动态分配。NekSSH 连接成功后，会把每个在线客户机的信息写入 ~/.nekssh/clients/*.json。不要猜测固定端口，也不要扫描端口。

## 先确认客户机在线

ls -1t "$HOME"/.nekssh/clients/*.json
cat "$HOME"/.nekssh/clients/<选择的记录>.json

读取 JSON 的 control 字段（形如 http://127.0.0.1:动态端口），然后访问：

curl -fsS "$CONTROL/"
curl -fsS "$CONTROL/docs"

/ 返回 JSON，service 为 nekssh、ok 为 true 且 control 与记录一致才继续。存在多份记录时逐一验证，只使用当前在线且与用户目标相符的客户机。/docs 是完整接口说明。全部连接失败就停下来，让用户检查 NekSSH 连接；不要扫描端口，也不要在服务器上启动服务充数。

## 步骤

1. 确认要运行的程序、启动命令，以及它监听的端口。端口必须是客户机上的 127.0.0.1 端口。用户没指定端口时，选一个大于 1024 的空闲端口，并让程序听 127.0.0.1 的这个端口。程序若读取环境变量，客户机上的 PORT 会设成这个值。
2. 把运行所需的文件写到客户机。路径相对于客户机的 workspace 目录，用 / 分隔。禁止 .. 。

curl -fsS "$CONTROL/v1/files" -H "Content-Type: application/json" -d "{\"files\":[{\"path\":\"app.py\",\"content\":\"print(1)\"}]}"

二进制文件把内容放进 contentBase64，不要放进 content。文件很多时按目录分批提交，每批保持在几十 MB 以内。

3. 在客户机启动，并反射端口。command 在客户机的 workspace 里执行。port 与程序监听端口相同。服务器侧端口由 SSH 动态分配，必须使用响应中的 remote 地址。

curl -fsS "$CONTROL/v1/run" -H "Content-Type: application/json" -d "{\"command\":\"python app.py\",\"port\":3000}"

4. 读返回 JSON。记下 id、pid、local、remote。ok 不是 true，或存在 reflectError，就查看日志并停止这个任务，不要告诉用户已经可以访问。
5. 在这台服务器上访问 remote 里的地址，确认服务有响应。再把这个地址告诉用户。
6. 需要输出时读取日志。需要结束时停止任务。停止会结束客户机上的整个进程树。

curl -fsS "$CONTROL/v1/jobs/job-1/log"
curl -fsS -X POST "$CONTROL/v1/jobs/job-1/stop"

已有任务列表：curl -fsS "$CONTROL/v1/jobs"

## 约束

- 文件只写到客户机 workspace 里面。启动命令只做用户要求的那件事。
- 反射端口只供这台服务器本机使用。把 remote 地址交给用户，不要把它写成公网地址。
- 客户机同一个业务端口同一时间只运行一个服务。服务器侧控制口和业务反射端口都动态分配，不会与其他用户争用固定端口。
- 不在客户机上做与用户请求无关的操作。用户要求停止时调用对应任务的 /stop。
`

type agentClientInfo struct {
	Service     string `json:"service"`
	Session     string `json:"session"`
	Client      string `json:"client"`
	Platform    string `json:"platform"`
	Control     string `json:"control"`
	Docs        string `json:"docs"`
	ConnectedAt string `json:"connectedAt"`
	PID         int    `json:"pid"`
}

func (a *App) installAgentSkill(session *desktopSession, sessionID, remoteAddress string) {
	if session == nil || session.client == nil {
		return
	}
	home, err := remoteHome(session.client)
	if err != nil {
		debugf("agent skill home: %v", err)
		return
	}
	client, err := sftp.NewClient(session.client)
	if err != nil {
		debugf("agent skill sftp: %v", err)
		return
	}
	defer client.Close()
	skillDir := home + "/" + agentSkillDir
	clientsDir := home + "/" + agentClientsDir
	if err = client.MkdirAll(skillDir); err != nil {
		debugf("agent skill mkdir: %v", err)
		return
	}
	if err = client.MkdirAll(clientsDir); err != nil {
		debugf("agent client mkdir: %v", err)
		return
	}
	_ = client.Chmod(clientsDir, 0700)
	target := skillDir + "/" + agentSkillFile
	file, err := client.Create(target)
	if err != nil {
		debugf("agent skill create: %v", err)
		return
	}
	_, err = file.Write([]byte(agentSkillMarkdown))
	file.Close()
	if err != nil {
		debugf("agent skill write: %v", err)
		return
	}
	clientName, _ := os.Hostname()
	info := agentClientInfo{Service: "nekssh-client", Session: sessionID, Client: clientName, Platform: "windows", Control: "http://" + remoteAddress, Docs: "http://" + remoteAddress + "/docs", ConnectedAt: time.Now().UTC().Format(time.RFC3339), PID: os.Getpid()}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return
	}
	sum := sha256.Sum256([]byte(sessionID + "\x00" + clientName + "\x00" + remoteAddress))
	infoPath := clientsDir + "/" + hex.EncodeToString(sum[:8]) + ".json"
	tempPath := infoPath + ".tmp"
	infoFile, err := client.Create(tempPath)
	if err != nil {
		debugf("agent client create: %v", err)
		return
	}
	_, err = infoFile.Write(append(data, '\n'))
	infoFile.Close()
	if err != nil {
		debugf("agent client write: %v", err)
		return
	}
	_ = client.Chmod(tempPath, 0600)
	if err = client.PosixRename(tempPath, infoPath); err != nil {
		_ = client.Remove(infoPath)
		if err = client.Rename(tempPath, infoPath); err != nil {
			_ = client.Remove(tempPath)
			debugf("agent client rename: %v", err)
			return
		}
	}
	session.setAgentInfoPath(infoPath)
	select {
	case <-session.done:
		_ = client.Remove(infoPath)
		return
	default:
	}
	debugf("agent skill installed at %s; client registered at %s", target, infoPath)
}

func (s *desktopSession) setAgentInfoPath(path string) {
	s.reflectMu.Lock()
	s.agentInfoPath = path
	s.reflectMu.Unlock()
}

func (s *desktopSession) removeAgentInfo() {
	s.reflectMu.Lock()
	path := s.agentInfoPath
	s.agentInfoPath = ""
	s.reflectMu.Unlock()
	if path == "" {
		return
	}
	s.sftpMu.Lock()
	defer s.sftpMu.Unlock()
	if s.sftp == nil {
		return
	}
	if err := s.sftp.Remove(path); err != nil && !os.IsNotExist(err) {
		debugf("agent client remove: %v", err)
	}
}

func remoteHome(client *ssh.Client) (string, error) {
	shell, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer shell.Close()
	var output bytes.Buffer
	shell.Stdout = &output
	if err = shell.Run("printf %s \"$HOME\""); err != nil {
		return "", err
	}
	home := strings.TrimSpace(output.String())
	if home == "" || strings.Contains(home, "..") || !strings.HasPrefix(home, "/") {
		return "", errString("远程 HOME 无效")
	}
	return home, nil
}
