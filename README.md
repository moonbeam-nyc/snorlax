<div align="center">
  <img src="./proxy/static/logo-small.png" alt="Logo" width="300">
</div>

# Snorlax

Snorlax is a Kubernetes operator which wakes and sleeps another Kubernetes deployment on a schedule.

And if a request is received when the deployment is sleeping, a cute sleeping Snorlax page is
served and the Kubernetes deployment is woken up. Once the service is ready, the page will auto-refresh.

You create `SleepSchedule` resources to define the schedule for any deployment (and optionally its ingress).


## Why Snorlax?

Sleeping your environments is the equivalent of turning off the lights at night.

- **Cost savings**: Scale down your environments when they're not needed (e.g. overnight), freeing up cloud resources
- **Security**: Reduce the attack surface of your infrastructure when deployments are sleeping
- **Environmentally responsible**: Reduce the energy consumption of your infrastructure

As a common example, if you sleep all of your staging/ephemeral deployments 8
hours each night and on weekends, that means they'll sleep ~55% of the month.
**That means ~55% savings on your cloud bill for those resources.**


## See it in action

![Snorlax Demo](./proxy/static/demo.gif)


## Usage

1. Install the `snorlax` Helm chart to install the `SleepSchedule` CRD and controller
    ```bash
    helm repo add moon-society https://moon-society.github.io/helm-charts
    helm repo update
    helm install snorlax moon-society/snorlax --create-namespace --namespace snorlax
    ```

2. Create your `SleepSchedule` resource to define the schedule for the deployment
    ```yaml
    # filename: your-app-sleep-schedule.yaml

    apiVersion: snorlax.moon-society.io/v1beta1
    kind: SleepSchedule
    metadata:
      namespace: your-app-namespace
      name: your-app
    spec:
      # Required fields
      wakeTime: '8:00am'
      sleepTime: '10:00pm'
      deploymentName: your-app-deployment
      wakeReplicas: 3

      # (optional) the timezone to use for the input times above
      timezone: 'America/New_York'

      # (optional) the ingress to update and point to the snorlax wake proxy,
      # which wakes your deployment when a request is received while it's
      # sleeping.
      ingressName: your-app-ingress
    ```

3. Apply the `SleepSchedule` resource
    ```bash
    kubectl apply -f your-app-sleep-schedule.yaml
    ```

## Other features

- Ingress controller awareness: Snorlax determines which ingress controller you're running so it can create the correct ingress routes

## Try it yourself locally

(Requires `minikube` and `helm` to be installed)

Run `make demo` to create a Minikube cluster, install the latest Helm release of `snorlax`, and deploy a dummy deployment, service, ingress, and sleep schedule.

Once you deploy the resources, you can update the sleep schedule with `kubectl edit sleepschedule dummy`.

## How to develop

Setup Minikube with the CRD and dummy application with sleep schedule:
```bash
make dev-setup
```

Then make your updates and run the operator:
```bash
make operator-run
```

## Future work

- Scale entire namespaces
- Sleep when no requests are received for a certain period of time
- Add support for custom wake and sleep actions (e.g. hit a webhook on wake)
- Add support for cron-style schedules (e.g. `0 8 * * *`)
- Add button to manually wake up the deployment (instead of auto-waking on request)
- Custom image/gif for sleeping page