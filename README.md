# NekSSH

一个简洁的 Web SSH 客户端：主机管理、密码/私钥认证、多标签交互终端、窗口尺寸同步和 SFTP 文件管理。

项目同时保留浏览器服务端和 Wails 桌面客户端。桌面端工程位于 `desktop/`，共享核心的重构说明见 `docs/ARCHITECTURE.md`。

## 运行

```bash
go mod download
go run .
```

访问 <http://127.0.0.1:8022>。终端组件已内置，可离线使用。

默认严格校验 `~/.ssh/known_hosts`。先用系统 SSH 连接一次目标服务器并确认指纹：

```bash
ssh user@example.com
```

仅限本地开发时，也可临时关闭校验：

```bash
NEKSSH_INSECURE_HOST_KEY=1 go run .
```

可通过 `NEKSSH_ADDR` 修改监听地址，通过 `NEKSSH_KNOWN_HOSTS` 指定主机指纹文件。

## RPM 安装包

构建：

```bash
./packaging/build-rpm.sh
```

安装：

```bash
sudo rpm -Uvh dist/nekssh-0.1.0-1.x86_64.rpm
```

安装后由 `nekssh.service` 管理并自动启动。配置文件位于 `/etc/nekssh/nekssh.env`，主机指纹保存在 `/var/lib/nekssh/known_hosts`。

## 安全说明

- 主机地址与用户名保存在浏览器 LocalStorage。
- 密码、私钥和私钥口令只用于当前连接，不会保存。
- 文件上传和下载复用当前 SSH/SFTP 会话，首版单文件限制为 10 MB。
- 默认只监听 `127.0.0.1`，不要在没有身份认证和 TLS 的情况下暴露到公网。
