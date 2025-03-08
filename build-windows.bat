@echo off
setlocal

if "%1"=="intel86" (
    set GOOS=windows
    set GOARCH=386
    set CGO_ENABLED=1
    set CC=gcc
    echo Memulai build...
    go build -ldflags "-s -w" -o build\windows\kirimkan.exe main.go
    echo Build selesai: build\windows\kirimkan.exe
    exit /b 0
)

if "%1"=="amd64" (
    set GOOS=windows
    set GOARCH=amd64
    set CGO_ENABLED=1
    set CC=gcc
    echo Memulai build...
    go build -ldflags "-s -w" -o build\windows\kirimkan.exe main.go
    echo Build selesai: build\windows\kirimkan.exe
    exit /b 0
)

echo Penggunaan: %~nx0
echo Contoh:
echo    %~nx0 intel86   - Build untuk Windows 32-bit (Intel x86)
echo    %~nx0 amd64     - Build untuk Windows 64-bit (AMD64)
exit /b 1