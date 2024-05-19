all: minikuve-start dummy-install operator-run

build:
	go build -o snorlax

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

minikube-start:
	minikube start --addons ingress

dummy-install:
	kubectl apply -f dummy-app/k8s.yaml

operator-install:
	cd operator && make install

operator-run:
	cd operator && make run