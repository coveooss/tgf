SOURCES = $(wildcard **/*.go)

install:
	go install

coveralls:
	wget https://raw.githubusercontent.com/coveo/terragrunt/master/scripts/coverage.sh
	@sh ./coverage.sh --coveralls
	rm coverage.sh

.PHONY: test
test:
	go test ./...

tgf: $(SOURCES)
	go build ./...
