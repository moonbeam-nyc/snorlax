OPERATOR_IMG = ghcr.io/moon-society/snorlax-operator:latest
BUNDLE_IMG = ghcr.io/moon-society/snorlax-operator-bundle:latest

all: prepare operator-run
setup: minikube-delete minikube-start proxy-install operator-crd-install sleep dummy-install


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

proxy-build:
	docker compose build snorlax

proxy-install: proxy-build
	docker save ghcr.io/moon-society/snorlax-proxy | (eval $$(minikube docker-env) && docker load)

proxy-serve: docker-build
	docker run -p 8080:8080 snorlax serve


## Minikube commands

minikube-start:
	minikube start --addons ingress

minikube-delete:
	minikube delete


## Dummy app

dummy-install:
	kubectl apply -f dummy-app/k8s.yaml


## Operator CRD

operator-crd-install:
	cd operator && make install


## Operator

operator: operator-build operator-push operator-deploy

operator-build:
	cd operator && make docker-build IMG=$(OPERATOR_IMG)

operator-push:
	cd operator && make docker-push IMG=$(OPERATOR_IMG)

operator-deploy:
	cd operator && make deploy IMG=$(OPERATOR_IMG)

operator-run:
	cd operator && make run


## Operator bundle

operator-bundle: operator-bundle-init operator-bundle-build operator-bundle-push operator-bundle-deploy

operator-bundle-init:
	cd operator && make bundle

operator-bundle-build:
	cd operator && make bundle-build BUNDLE_IMG=$(BUNDLE_IMG)

operator-bundle-push:
	cd operator && make bundle-push BUNDLE_IMG=$(BUNDLE_IMG)

operator-bundle-deploy:
	cd operator && operator-sdk run bundle $(BUNDLE_IMG)