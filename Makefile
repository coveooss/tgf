SOURCES = $(wildcard **/*.go)

install:
	go install

.PHONY: test
test:
	go test ./...

tgf: $(SOURCES) go.mod go.sum
	go build ./...
