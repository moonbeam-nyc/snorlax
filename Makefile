all: dev

build:
	go build -o snorlax

help: build
	./snorlax

serve: build
	./snorlax serve

watch: build
	./snorlax watch

wake: build
	./snorlax wake

sleep: build
	./snorlax sleep

dev:
	air

clean:
	rm -f snorlax

docker-build:
	docker build -t snorlax .

docker-run: docker-build
	docker run -p 8080:8080 snorlax

minikube-push: docker-build
	docker save snorlax | (eval $$(minikube docker-env) && docker load)

minikube-install:
	helm upgrade -i snorlax-nginx charts/snorlax -n default --create-namespace

minikube-uninstall:
	helm uninstall snorlax-nginx -n default