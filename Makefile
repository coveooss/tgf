SOURCES = $(wildcard **/*.go)

install:
	go install

.PHONY: test
test:
	go test ./...

tgf: $(SOURCES)
	go build ./...

.PHONY: build
build: terraform-provider-quantum

docker:
	bash make_dockers.sh
