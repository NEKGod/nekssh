@echo off
setlocal
cd /d "%~dp0"
if not exist data mkdir data
set "NEKSSH_ADDR=0.0.0.0:8022"
set "NEKSSH_KNOWN_HOSTS=%~dp0data\known_hosts"
if not exist "%NEKSSH_KNOWN_HOSTS%" type nul > "%NEKSSH_KNOWN_HOSTS%"
netsh advfirewall firewall add rule name="NekSSH 8022" dir=in action=allow protocol=TCP localport=8022 >nul 2>&1
start "" cmd /c "timeout /t 2 /nobreak >nul & start http://127.0.0.1:8022"
nekssh.exe
endlocal
