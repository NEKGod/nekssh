$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ProjectRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 Go，请先安装 Go 1.23 或更高版本。"
}
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    throw "未找到 Node.js，请先安装 Node.js 22 LTS。"
}
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "正在安装 Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
    $env:Path += ";$env:USERPROFILE\go\bin"
}

Write-Host "正在安装前端依赖..."
Push-Location frontend
npm install
Pop-Location

$versionFile = Join-Path $ProjectRoot "version.go"
$versionText = Get-Content -Raw -Encoding UTF8 $versionFile
if ($versionText -notmatch 'const productVersion = "(\d+)\.(\d+)\.(\d+)"') {
    throw "version.go 里没有 productVersion"
}
$nextVersion = "{0}.{1}.{2}" -f $Matches[1], $Matches[2], ([int]$Matches[3] + 1)
$versionText = $versionText -replace 'const productVersion = "\d+\.\d+\.\d+"', "const productVersion = `"$nextVersion`""
[System.IO.File]::WriteAllText($versionFile, $versionText.TrimEnd() + "`n", (New-Object System.Text.UTF8Encoding $false))
Write-Host "版本号：$nextVersion"

Write-Host "正在运行 Go 测试..."
go test ./...

Write-Host "正在构建 Windows x64 客户端和 NSIS 安装程序..."
wails build -clean -platform windows/amd64 -nsis

$Output = Join-Path $ProjectRoot "build\bin"
Write-Host "构建完成：$Output"
Get-ChildItem $Output
