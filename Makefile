SOURCES = $(wildcard **/*.go)

install:
	glide install
	go install

.PHONY: test
test:
	go test ./...

tgf: $(SOURCES)
	go build ./...
