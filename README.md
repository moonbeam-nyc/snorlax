<div align="center">
  <img src="./wake-server/static/logo-small.png" alt="Logo" width="300">
</div>

# Snorlax · [![Build Docker image](https://github.com/moonbeam-nyc/snorlax/actions/workflows/build-docker-image.yaml/badge.svg)](https://github.com/moonbeam-nyc/snorlax/actions/workflows/build-docker-image.yaml)

**Snorlax** is a Kubernetes operator that wakes and sleeps a specified set of Kubernetes deployments on a schedule.

You can also specify ingresses which will be updated to point to a wake server when asleep. When a request is received, **the wake server serves a "waking up" splash page and wakes the deployments up**.
Once the deployments are ready, the ingresses are restored and the splash page will auto-refresh.


## Why Snorlax?

Sleeping your environments is the equivalent of turning off the lights at night.

- **Cost savings**: Scale down your environments when they're not needed (e.g. overnight), freeing up cloud resources
- **Security**: Reduce the attack surface of your infrastructure when deployments are sleeping
- **Environmentally responsible**: Reduce the energy consumption of your infrastructure

As a common example, if you sleep all of your staging/ephemeral deployments for 8 hours each night and on weekends, they'll sleep ~55% of the month.
**That means ~55% savings on your cloud bill for those resources.**


## See it in action

![Snorlax Demo](./wake-server/static/demo.gif)


## Usage

1. Install the `snorlax` Helm chart to install the `SleepSchedule` CRD and controller
    ```bash
    helm repo add moonbeam https://moonbeam-nyc.github.io/helm-charts
    helm repo update
    helm install snorlax moonbeam/snorlax --create-namespace --namespace snorlax
    ```

2. Create your `SleepSchedule` resource to define the schedule for the deployment
    ```yaml
    # filename: your-app-sleep-schedule.yaml

    apiVersion: snorlax.moonbeam.nyc/v1beta1
    kind: SleepSchedule
    metadata:
      namespace: your-app-namespace
      name: your-app
    spec:
      # Required fields
      wakeTime: '8:00am'
      sleepTime: '10:00pm'
      deployments:
      - name: your-app-frontend
      - name: your-app-db
      - name: your-app-redis

      # (optional) the ingresses to update and point to the snorlax wake server,
      # which wakes your deployment when a request is received while it's
      # sleeping.
      ingresses:
      - name: your-app-ingress

        # (optional, defaults to all deployments) specify which deployments
        # must be ready to wake this ingress
        requires:
        - deployment:
            name: your-app-frontend

      # (optional, defaults to UTC) the timezone to use for the input times above
      timezone: 'America/New_York'
      # (optional), Set the resource request and limits of the snorlax deployment
      resources:
        requests:
          cpu: 10m
          memory: 64Mi
        limits:
          cpu: 50m
          memory: 128Mi
    ```

3. Apply the `SleepSchedule` resource
    ```bash
    kubectl apply -f your-app-sleep-schedule.yaml
    ```

## Other features

- **Ingress controller awareness**: Snorlax determines which ingress controller you're running so it can create the correct ingress routes for sleep.
- **Stays awake until next sleep cycle**: If a request is received during the sleep time, the deployment will stay awake until the next sleep cycle
- **Ignores ELB health checks**: Snorlax ignores health checks from ELBs so that they don't wake up the deployment

## Try it yourself locally

(Requires `make`, `minikube` and `helm` to be installed)

Run `make demo` to:
- create a Minikube cluster
- install the latest Helm release of `snorlax`
- deploy a dummy deployment, service, ingress, and sleep schedule
- starts the minikube tunnel to proxy `localhost` to your Minikube cluster ingress service (you'll need to enter your password)

Then go to [http://localhost](http://localhost) to see either the sleeping page or the dummy deployment (depending on the time of day).

You can also then try updating the sleep schedule with `kubectl edit sleepschedule dummy`.

## How to develop

(Requires `make`, `minikube`, `helm`, and `docker` to be installed)

Setup Minikube with the CRD and dummy application with sleep schedule:
```bash
make dev-setup
```

Then make your updates and run the operator:
```bash
make dev-run
```

## Future work

- Scale entire namespaces
- Sleep when no requests are received for a certain period of time
- Add support for custom wake & sleep actions (e.g. hit a webhook on wake)
- Add support for cron-style schedules (e.g. `0 8 * * *`)
- Add a button to manually wake up the deployment (instead of auto-waking on request)
- Custom image/gif for sleeping page
- Always sleeping mode, reset at a certain time of day
- Support waking a deployment on TCP connection
- Select deployments & ingresses by label
- Support wildcards for deployment & ingress names

## Reach out

Wondering how to best use Snorlax? Have questions or ideas for new features?

I'd love to hear, and maybe even build them! Reach out to me at:

<img src="./wake-server/static/readme-email-address.png" alt="Contact" width="300">
