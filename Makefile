build:
	GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o bin/mirror-git \
		cmd/mirror-git/main.go
