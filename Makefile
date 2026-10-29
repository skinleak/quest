BIN := bin/quest

.PHONY: build run test race bench lint fmt vet check clean

build:
	go build -o $(BIN) ./cmd/quest

run:
	go run ./cmd/quest

test:
	go test ./...

race:
	go test -race ./...

bench:
	go test -run '^$$' -bench . -benchmem ./internal/...

lint:
	golangci-lint run

fmt:
	gofmt -w .

vet:
	go vet ./...

# The definition of done: formatted, vetted, built, tested, linted.
check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && false)
	go vet ./...
	go build ./...
	go test ./...
	golangci-lint run

clean:
	rm -rf bin
