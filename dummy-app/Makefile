IMAGE=python-server

all: build push deploy

build:
	docker build -t $(IMAGE) .

run:
	docker run -it -p 8000:8000 $(IMAGE)

push:
	docker save $(IMAGE) | (eval $$(minikube docker-env) && docker load)

deploy:
	kubectl apply -f k8s.yaml
