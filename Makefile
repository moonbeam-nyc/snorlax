all: dev

build:
	go build -o snorlax

help: build
	./snorlax

serve: build
	./snorlax serve

watch: build
	./snorlax watch

dev:
	air

clean:
	rm -f snorlax

docker-build:
	docker build -t snorlax .

docker-run: docker-build
	docker run -p 8080:8080 snorlax