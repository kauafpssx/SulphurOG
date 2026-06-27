.PHONY: build build-linux auth run clean vet lint tidy

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/sulphurog.exe ./cmd/sulphurog/

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/sulphurog-linux ./cmd/sulphurog/

auth:
	go run ./cmd/auth/

run:
	go run ./cmd/sulphurog/

clean:
	rm -rf bin/ *.exe

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
