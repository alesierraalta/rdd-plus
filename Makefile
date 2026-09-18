.PHONY: build test mutants vet

build:
	go build -o bin/rdd-plus ./cmd/rdd-plus

test:
	go test ./... -count=1

mutants:
	go run ./tools/mutants

vet:
	test -z "$$(gofmt -l .)" && go vet ./...
