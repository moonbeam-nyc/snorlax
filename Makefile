all: prepare operator-run
prepare: minikube-start minikube-push operator-install sleep dummy-install




## Local commands

build:
	go build -o snorlax

help: build
	./snorlax

serve: build
	./snorlax serve

clean:
	rm -f snorlax

sleep:
	sleep 10


## Docker commands

docker-build:
	docker compose build snorlax

docker-serve: docker-build
	docker run -p 8080:8080 snorlax serve


## Minikube commands

minikube-push: docker-build
	docker save ghcr.io/moon-society/snorlax | (eval $$(minikube docker-env) && docker load)

minikube-start:
	minikube start --addons ingress

minikube-delete:
	minikube delete


## Dummy app

dummy-install:
	kubectl apply -f dummy-app/k8s.yaml


## Operator

operator-install:
	cd operator && make install

operator-run:
	cd operator && make run