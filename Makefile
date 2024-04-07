build:
	go build -o snorlax


## Local commands

help: build
	./snorlax

watch-serve: build
	./snorlax watch-serve

wake: build
	./snorlax wake

sleep: build
	./snorlax sleep

clean:
	rm -f snorlax


## Docker commands

docker-build:
	docker build -t snorlax .

docker-watch-serve: docker-build
	docker run -p 8080:8080 snorlax watch-serve


## Minikube commands

minikube-push: docker-build
	docker save snorlax | (eval $$(minikube docker-env) && docker load)

minikube-install:
	helm upgrade -i snorlax-nginx charts/snorlax -n default --create-namespace

minikube-uninstall:
	helm uninstall snorlax-nginx -n default