set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1
go build -ldflags "-s -w" -o kirimkan.exe main.go