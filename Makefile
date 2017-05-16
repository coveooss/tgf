install:
	go install

docker:
	docker build -f Dockerfile -t coveo/tgf .

all-dockers:
	docker build -f Dockerfile -t coveo/tgf .
	docker build -f Dockerfile.Shell -t coveo/tgf.shell .
	docker build -f Dockerfile.Full -t coveo/tgf.full .
