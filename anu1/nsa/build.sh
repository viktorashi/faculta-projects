clear && cd web-server/ && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o out/web-server .
