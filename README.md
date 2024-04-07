# Snorlax

Snorlax is a Kubernetes service which wakes and sleeps another Kubernetes on a schedule.

And if a request is received when the deployment is sleeping, a cute sleeping Snorlax page is
served and the Kubernetes deployment is woken up.

![Snorlax Banner](./static/snorlax-banner.webp)

## How to deploy to your cluster

Snorlax is packaged as a Helm chart. So create a Helm values file like so:

```yaml
# values.yaml

deployment:
  env:
    - name: REPLICA_COUNT
      value: "1"
    - name: NAMESPACE
      value: "default"
    - name: DEPLOYMENT_NAME
      value: "acme-service"
    - name: WAKE_TIME
      value: "8:00"
    - name: SLEEP_TIME
      value: "18:00"
    - name: INGRESS_NAME
      value: "nginx-ingress"

```

Then deploy it like so:

```bash
helm install snorlax ./snorlax --values values.yaml
```


## How to develop

If you have Go installed, you can build and run the program in full using:

```bash
make build watch-serve
```
