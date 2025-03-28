@echo off
setlocal

if "%1"=="intel86" (
    set GOOS=windows
    set GOARCH=386
    set CGO_ENABLED=1
    set CC=gcc
    echo Preparing intel86 Build
    if not exist "build\windows\intel86" mkdir "build\windows\intel86"
    echo Starting exe compilation...
    go build -ldflags "-s -w" -o build\windows\intel86\kirimkan-1.0.exe main.go
    echo Compilation completed: build\windows\intel86\kirimkan.exe
    echo Copying web files...
    xcopy web build\windows\intel86\web /E /Y /I
    echo Creating Configuration...
    echo API_HOST=127.0.0.1 > build\windows\intel86\kirimkan.conf
    echo API_PORT=7069 >> build\windows\intel86\kirimkan.conf
    echo API_TOKEN=983be0d548d7936377fd9f6279ae7c4f >> build\windows\intel86\kirimkan.conf
    echo Configuration saved: build\windows\intel86\kirimkan.conf
    echo Build Completed
    exit /b 0
)

if "%1"=="amd64" (
    set GOOS=windows
    set GOARCH=amd64
    set CGO_ENABLED=1
    set CC=gcc
    echo Preparing amd64 Build
    if not exist "build\windows\amd64" mkdir "build\windows\amd64"
    echo Starting exe compilation...
    go build -ldflags "-s -w" -o build\windows\amd64\kirimkan-1.0.exe main.go
    echo Compilation completed: build\windows\amd64\kirimkan.exe
    echo Copying web files...
    xcopy web build\windows\amd64\web /E /Y /I
    echo Creating Configuration...
    echo API_HOST=127.0.0.1 > build\windows\amd64\kirimkan.conf
    echo API_PORT=7069 >> build\windows\amd64\kirimkan.conf
    echo API_TOKEN=983be0d548d7936377fd9f6279ae7c4f >> build\windows\amd64\kirimkan.conf
    echo Configuration saved: build\windows\amd64\kirimkan.conf
    echo Build Completed
    exit /b 0
)

echo Usage: %~nx0
echo Example:
echo    %~nx0 intel86   - Build for Windows 32-bit (Intel x86)
echo    %~nx0 amd64     - Build for Windows 64-bit (AMD64)
exit /b 1