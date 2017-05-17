install:
	go install

deploy:
	GOOS=linux go build -o .pkg/tgf_linux
	GOOS=darwin go build -o .pkg/tgf_darwin
	GOOS=windows go build -o .pkg/tgf.exe

docker:
	docker build -f Dockerfile -t coveo/tgf .

all-dockers:
	docker build -f Dockerfile -t coveo/tgf .
	docker build -f Dockerfile.Shell -t coveo/tgf.shell .
	docker build -f Dockerfile.Full -t coveo/tgf.full .
