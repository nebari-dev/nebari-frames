.PHONY: proto lint test build clean

proto:
	cd proto && buf lint
	cd proto && buf generate

lint:
	golangci-lint run ./...

test:
	go test ./... -race -coverprofile=coverage.out

build:
	CGO_ENABLED=0 go build -o nebari-frames-server ./backend/cmd/server

clean:
	rm -f nebari-frames-server coverage.out *.db
