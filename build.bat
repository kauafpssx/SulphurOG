@echo off
echo Building sulphurog for Linux...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o sulphurog ./cmd/sulphurog
if %ERRORLEVEL% NEQ 0 (
    echo BUILD FAILED
    pause
    exit /b 1
)
echo Done: sulphurog (Linux amd64)
