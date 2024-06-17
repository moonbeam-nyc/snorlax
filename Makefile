VERSION = 0.6.0
BUNDLE_IMG = ghcr.io/moonbeam-nyc/snorlax-operator-bundle:${VERSION}
OPERATOR_IMG = ghcr.io/moonbeam-nyc/snorlax-operator:${VERSION}
WAKE_SERVER_IMG = ghcr.io/moonbeam-nyc/snorlax-wake-server:${VERSION}
PROXY_IMG = ghcr.io/moonbeam-nyc/snorlax-proxy:${VERSION}


## Workflows

dev-setup: minikube-delete minikube-start wake-server-install operator-crd-install dummy-install minikube-tunnel
dev-run: operator-run
demo: minikube-reset helm-install-remote dummy-install minikube-tunnel
release: wake-server-release-multiplatform operator-release-multiplatform operator-helmify helm-package
release-images: wake-server-release-multiplatform operator-release-multiplatform


## Local commands

local-proxy-build:
	cd proxy && go build -o snorlax-proxy

local-proxy-run: local-proxy-build
	cd proxy && ./snorlax-proxy

local-proxy-clean:
	cd proxy && rm -f snorlax-proxy


## Helm commands

helm-install-local:
	helm install snorlax ./charts/snorlax \
		--create-namespace \
		--namespace snorlax

helm-install-remote:
	helm repo add moonbeam https://moonbeam-nyc.github.io/helm-charts
	helm repo update
	helm install snorlax moonbeam/snorlax \
		--create-namespace \
		--namespace snorlax

helm-uninstall:
	helm uninstall snorlax --namespace snorlax

helm-package:
	cd ./charts/snorlax && helm package .
	mv ./charts/snorlax/snorlax-*.tgz .


## Wake server commands

wake-server-build:
	cd wake-server && VERSION=$(VERSION) docker compose build snorlax

wake-server-release: wake-server-build
	docker push $(WAKE_SERVER_IMG)

wake-server-release-multiplatform:
	- docker buildx create --use --name builder
	docker buildx use builder
	cd wake-server && docker buildx build --platform linux/amd64,linux/arm64 --tag $(WAKE_SERVER_IMG) --push .

wake-server-install: wake-server-build
	docker save $(WAKE_SERVER_IMG) | (eval $$(minikube docker-env) && docker load)

wake-server-serve: docker-build
	docker run -p 8080:8080 $(WAKE_SERVER_IMG) serve


## Proxy commands

proxy-build:
	cd proxy && VERSION=$(VERSION) docker compose build snorlax-proxy

proxy-run:
	cd proxy && VERSION=$(VERSION) docker compose up

proxy-release: proxy-build
	docker push $(PROXY_IMG)

proxy-release-multiplatform:
	- docker buildx create --use --name builder
	docker buildx use builder
	cd proxy && docker buildx build --platform linux/amd64,linux/arm64 --tag $(PROXY_IMG) --push .

proxy-install: proxy-build
	docker save $(PROXY_IMG) | (eval $$(minikube docker-env) && docker load)


## Minikube commands

minikube-reset: minikube-delete minikube-start

minikube-start:
	minikube start --addons ingress
	kubectl rollout status -n ingress-nginx deployment/ingress-nginx-controller

minikube-delete:
	minikube delete

minikube-tunnel:
	minikube tunnel


## Dummy app

dummy-install:
	kubectl apply -f dummy-app


## Operator CRD

operator-crd-install:
	cd operator && make install

operator-crd-uninstall:
	cd operator && make uninstall


## Operator

operator-build:
	cd operator && make docker-build IMG=$(OPERATOR_IMG)

operator-release: operator-build
	cd operator && make docker-push IMG=$(OPERATOR_IMG)

operator-release-multiplatform:
	cd operator && make docker-buildx IMG=$(OPERATOR_IMG)

operator-deploy:
	cd operator && make deploy IMG=$(OPERATOR_IMG)

operator-helmify:
	cd operator && make helmify IMG=$(OPERATOR_IMG)
	-rm -rf charts/snorlax
	mv operator/snorlax ./charts/snorlax
	VERSION=$(VERSION) yq eval ".version = env(VERSION)" -i charts/snorlax/Chart.yaml
	VERSION=$(VERSION) yq eval ".appVersion = env(VERSION)" -i charts/snorlax/Chart.yaml

operator-run:
	cd operator && make run


## Operator bundle

operator-bundle: operator-bundle-olm-install operator-bundle-init operator-bundle-build operator-bundle-push operator-bundle-deploy

operator-bundle-olm-install:
	operator-sdk olm install

operator-bundle-init:
	cd operator && make bundle

operator-bundle-build:
	cd operator && make bundle-build BUNDLE_IMG=$(BUNDLE_IMG)

operator-bundle-push:
	cd operator && make bundle-push BUNDLE_IMG=$(BUNDLE_IMG)

operator-bundle-deploy:
	cd operator && operator-sdk run bundle $(BUNDLE_IMG)
