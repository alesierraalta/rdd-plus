.PHONY: build test mutants vet

build:
	go build -o bin/tpp ./cmd/tpp

test:
	go test ./... -count=1

mutants:
	go run ./tools/mutants

vet:
	test -z "$$(gofmt -l .)" && go vet ./...
