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
	docker build -f Dockerfile -t coveo/tgf .

all-dockers:
	docker build -f Dockerfile -t coveo/tgf .
	docker build -f Dockerfile.Base -t coveo/tgf.base .
	docker build -f Dockerfile.Full -t coveo/tgf.full .
