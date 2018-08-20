SOURCES = $(wildcard **/*.go)

install:
	go install

.PHONY: test
test:
	go test ./...

tgf: $(SOURCES)
	go build ./...
