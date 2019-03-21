all: test


test:
	go test -race -v ./...
