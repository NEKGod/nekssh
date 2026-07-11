# 构建 Windows 桌面客户端

需要 Windows 10/11 x64、Go 1.23+、Node.js 22 LTS、WebView2 Runtime 和 NSIS。

在 PowerShell 中进入 `desktop` 目录后运行：

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\build-windows.ps1
```

输出位于 `build\bin`，包括 `NekSSH.exe` 和 NSIS 安装程序。

如果网络需要代理，可先设置：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:12000"
$env:HTTPS_PROXY="http://127.0.0.1:12000"
$env:GOPROXY="https://proxy.golang.org,direct"
```

安装 NSIS：

```powershell
winget install NSIS.NSIS
```
