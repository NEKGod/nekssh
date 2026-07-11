@echo off
setlocal
cd /d "%~dp0"
if not exist data mkdir data
set "NEKSSH_ADDR=127.0.0.1:8022"
set "NEKSSH_KNOWN_HOSTS=%~dp0data\known_hosts"
if not exist "%NEKSSH_KNOWN_HOSTS%" type nul > "%NEKSSH_KNOWN_HOSTS%"
start "" cmd /c "timeout /t 2 /nobreak >nul & start http://127.0.0.1:8022"
nekssh.exe
endlocal
