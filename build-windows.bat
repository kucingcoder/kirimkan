@echo off
setlocal

if "%1"=="intel86" (
    set GOOS=windows
    set GOARCH=386
    set CGO_ENABLED=1
    set CC=gcc
    echo Mempersiapkan Build intel86
    if not exist "build\windows\intel86" mkdir "build\windows\intel86"
    echo Memulai kompilasi exe...
    go build -ldflags "-s -w" -o build\windows\intel86\kirimkan-1.0.exe main.go
    echo Kompilasi selesai: build\windows\intel86\kirimkan.exe
    echo Menyalin web...
    xcopy web build\windows\intel86\web /E /Y /I
    echo Membuat Konfigurasi...
    echo API_HOST=127.0.0.1 > build\windows\intel86\kirimkan.conf
    echo API_PORT=7069 >> build\windows\intel86\kirimkan.conf
    echo API_TOKEN=983be0d548d7936377fd9f6279ae7c4f >> build\windows\intel86\kirimkan.conf
    echo Konfigurasi disimpan: build\windows\intel86\kirimkan.conf
    echo Build Selesai
    exit /b 0
)

if "%1"=="amd64" (
    set GOOS=windows
    set GOARCH=amd64
    set CGO_ENABLED=1
    set CC=gcc
    echo Mempersiapkan Build amd64
    if not exist "build\windows\amd64" mkdir "build\windows\amd64"
    echo Memulai kompilasi exe...
    go build -ldflags "-s -w" -o build\windows\amd64\kirimkan-1.0.exe main.go
    echo Kompilasi selesai: build\windows\amd64\kirimkan.exe
    echo Menyalin web...
    xcopy web build\windows\amd64\web /E /Y /I
    echo Membuat Konfigurasi...
    echo API_HOST=127.0.0.1 > build\windows\amd64\kirimkan.conf
    echo API_PORT=7069 >> build\windows\amd64\kirimkan.conf
    echo API_TOKEN=983be0d548d7936377fd9f6279ae7c4f >> build\windows\amd64\kirimkan.conf
    echo Konfigurasi disimpan: build\windows\amd64\kirimkan.conf
    echo Build Selesai
    exit /b 0
)

echo Penggunaan: %~nx0
echo Contoh:
echo    %~nx0 intel86   - Build untuk Windows 32-bit (Intel x86)
echo    %~nx0 amd64     - Build untuk Windows 64-bit (AMD64)
exit /b 1