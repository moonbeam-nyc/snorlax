/*
Copyright 2024 Peter Valdez.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	snorlaxv1beta1 "moonbeam-nyc/snorlax/api/v1beta1"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SleepScheduleReconciler reconciles a SleepSchedule object
type SleepScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const finalizer = "finalizer.snorlax.moonbeam.nyc"

//+kubebuilder:rbac:groups=snorlax.moonbeam.nyc,resources=sleepschedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snorlax.moonbeam.nyc,resources=sleepschedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=snorlax.moonbeam.nyc,resources=sleepschedules/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;watch;list;create;update;delete;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;watch;list;scale;update;create;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;watch;list;update;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;watch;list;create;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;watch;list;create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;watch;list;create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;watch;list;create

func (r *SleepScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the SleepSchedule instance
	sleepSchedule := &snorlaxv1beta1.SleepSchedule{}
	err := r.Get(ctx, req.NamespacedName, sleepSchedule)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)
	location, err := time.LoadLocation(sleepSchedule.Spec.Timezone)
	if err != nil {
		log.Error(err, "failed to load timezone")
		return ctrl.Result{}, err
	}
	now := time.Now().In(location)

	// Parse the wake time
	wakeTime, err := time.Parse("3:04pm", sleepSchedule.Spec.WakeTime)
	if err != nil {
		log.Error(err, "failed to parse wake time")
		return ctrl.Result{}, err
	}

	// Parse the sleep time
	sleepTime, err := time.Parse("3:04pm", sleepSchedule.Spec.SleepTime)
	if err != nil {
		log.Error(err, "failed to parse sleep time")
		return ctrl.Result{}, err
	}

	// Load the timezone
	var timezone *time.Location
	if sleepSchedule.Spec.Timezone != "" {
		var err error
		timezone, err = time.LoadLocation(sleepSchedule.Spec.Timezone)
		if err != nil {
			log.Error(err, "failed to load time zone")
			return ctrl.Result{}, err
		}
	} else {
		timezone = time.UTC
	}

	wakeDatetime := time.Date(now.Year(), now.Month(), now.Day(), wakeTime.Hour(), wakeTime.Minute(), 0, 0, timezone)
	sleepDatetime := time.Date(now.Year(), now.Month(), now.Day(), sleepTime.Hour(), sleepTime.Minute(), 0, 0, timezone)

	awake, err := r.isAppAwake(ctx, sleepSchedule)
	if err != nil {
		log.Error(err, "Failed to determine if the application is awake")
		return ctrl.Result{}, err
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(sleepSchedule, finalizer) {
		controllerutil.AddFinalizer(sleepSchedule, finalizer)
		err = r.Update(ctx, sleepSchedule)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status based on the actual check
	sleepSchedule.Status.Awake = awake
	err = r.Status().Patch(ctx, sleepSchedule, client.MergeFrom(sleepSchedule.DeepCopy()))
	if err != nil {
		log.Error(err, "failed to patch SleepSchedule status")
		return ctrl.Result{}, err
	}

	// Check if the instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	markedForDeletion := sleepSchedule.GetDeletionTimestamp() != nil
	if markedForDeletion {
		if controllerutil.ContainsFinalizer(sleepSchedule, finalizer) {
			// Run finalization logic for finalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeSleepSchedule(ctx, sleepSchedule); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Determine if the app should be awake or asleep
	var shouldSleep bool
	if wakeDatetime.Before(sleepDatetime) {
		shouldSleep = now.Before(wakeDatetime) || now.After(sleepDatetime)
	} else {
		shouldSleep = now.After(sleepDatetime) && now.Before(wakeDatetime)
	}

	// If the app should be awake, clear the proxy data
	if !shouldSleep {
		configMap := &corev1.ConfigMap{}
		err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: fmt.Sprintf("%s-sleep-data", objectName)}, configMap)
		if err == nil {
			err = r.Delete(ctx, configMap)
			if err != nil {
				log.Error(err, "failed to delete sleep-data configmap")
				return ctrl.Result{}, err
			}
		}
	}

	// fmt.Println("Checking if the app should be awake or asleep")
	// fmt.Println("now:", now)
	// fmt.Println("wakeDatetime:", wakeDatetime)
	// fmt.Println("sleepDatetime:", sleepDatetime)
	// fmt.Print("shouldSleep:", shouldSleep, "\n\n")

	// Check if the configmaps request-received key was set to "true"
	var wakeRequestReceived bool
	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: fmt.Sprintf("%s-sleep-data", objectName)}, configMap)
	if err != nil {
		// log.Error(err, "Failed to get ConfigMap")
		wakeRequestReceived = false
	} else {
		wakeRequestReceived = configMap.Data["received-request"] == "true"
	}

	// log.Info(fmt.Sprintf("wakeRequestReceived: %t", wakeRequestReceived))

	if awake && shouldSleep && !wakeRequestReceived {
		log.Info("Going to sleep")
		r.sleep(ctx, sleepSchedule)
	} else if !awake && (!shouldSleep || wakeRequestReceived) {
		log.Info("Waking up")
		r.wake(ctx, sleepSchedule)
	}

	// Requeue to check again in 10 seconds
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *SleepScheduleReconciler) finalizeSleepSchedule(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) error {
	log := log.FromContext(ctx)

	log.Info("finalizing the sleepschedule, waking the deployment")

	// Wake the environment
	if !sleepSchedule.Status.Awake {
		err := r.wake(ctx, sleepSchedule)
		if err != nil {
			return err
		}
	}

	// Remove the finalizer from the SleepSchedule
	controllerutil.RemoveFinalizer(sleepSchedule, finalizer)
	err := r.Update(context.TODO(), sleepSchedule)
	if err != nil {
		return err
	}

	return nil
}

func (r *SleepScheduleReconciler) isAppAwake(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) (bool, error) {
	// Return false if the sleep schedule has no deployments
	if len(sleepSchedule.Spec.DeploymentNames) == 0 {
		return false, nil
	}

	// Return false if any deployment has 0 replicas
	for _, deploymentName := range sleepSchedule.Spec.DeploymentNames {
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: deploymentName}, deployment)
		if err != nil {
			return false, err
		}

		if *deployment.Spec.Replicas == 0 {
			return false, nil
		}
	}

	return true, nil
}

func (r *SleepScheduleReconciler) wake(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) error {
	// Scale up each deployment
	var wg sync.WaitGroup
	for _, deploymentName := range sleepSchedule.Spec.DeploymentNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Get the number of replicas to scale up to
			replicas, err := r.getDeploymentReplicas(ctx, sleepSchedule, name)
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to get deployment replicas")
				replicas = 1
			}

			// Wake and wait for the deployment
			r.scaleDeployment(ctx, sleepSchedule.Namespace, name, replicas)
			r.waitForDeploymentToWake(ctx, sleepSchedule.Namespace, name)
		}(deploymentName)
	}

	// Wait for all deployments to finish scaling up
	wg.Wait()

	// Load the ingress copies
	for _, ingressName := range sleepSchedule.Spec.IngressNames {
		err := r.loadIngressCopy(ctx, sleepSchedule, ingressName)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to load Ingress copy")
			return err
		}
	}

	// Delete the Snorlax proxy
	err := r.DeleteSnorlaxProxy(ctx, sleepSchedule)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to delete Snorlax proxy")
		return err
	}

	return nil
}

func (r *SleepScheduleReconciler) sleep(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) {
	// Deploy the Snorlax proxy
	r.deploySnorlaxProxy(ctx, sleepSchedule)

	// Point each ingress to the Snorlax proxy
	for _, ingressName := range sleepSchedule.Spec.IngressNames {
		r.takeIngressCopy(ctx, sleepSchedule, ingressName)
		r.pointIngressToSnorlax(ctx, sleepSchedule, ingressName)
	}

	// Scale down each deployment
	for _, deploymentName := range sleepSchedule.Spec.DeploymentNames {
		r.storeCurrentReplicas(ctx, sleepSchedule, deploymentName)
		r.scaleDeployment(ctx, sleepSchedule.Namespace, deploymentName, 0)
	}
}

func (r *SleepScheduleReconciler) getDeploymentReplicas(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule, deploymentName string) (int32, error) {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: deploymentName}, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Deployment")
		return 0, err
	}

	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: fmt.Sprintf("%s-sleep-data", objectName)}, configMap)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ConfigMap")
		return 0, err
	}

	replicasKey := fmt.Sprintf("replicas.%s", deploymentName)
	replicasValue := configMap.Data[replicasKey]
	replicas, err := strconv.Atoi(replicasValue)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to parse replicas")
		return 0, err
	}

	return int32(replicas), nil
}

func (r *SleepScheduleReconciler) storeCurrentReplicas(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule, deploymentName string) {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: deploymentName}, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Deployment")
		return
	}

	replicasKey := fmt.Sprintf("replicas.%s", deploymentName)
	configMapData := map[string]string{
		replicasKey: fmt.Sprintf("%d", *deployment.Spec.Replicas),
	}

	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: fmt.Sprintf("%s-sleep-data", objectName)}, configMap)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ConfigMap")
		return
	}

	// Prepare the patch
	patchData := map[string]interface{}{
		"data": configMapData,
	}
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to marshal patch data")
		return
	}

	err = r.Patch(ctx, configMap, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update ConfigMap")
		return
	}
}

func (r *SleepScheduleReconciler) scaleDeployment(ctx context.Context, namespace, deploymentName string, replicas int32) {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Deployment")
		return
	}

	deployment.Spec.Replicas = &replicas
	err = r.Update(ctx, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Deployment replicas")
	}
}

func (r *SleepScheduleReconciler) waitForDeploymentToWake(ctx context.Context, namespace, deploymentName string) {
	logger := log.FromContext(ctx)

	for {
		logger.Info(fmt.Sprintf("Waiting for deployment to wake: %s", deploymentName))
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to get Deployment")
			return
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			logger.Info(fmt.Sprintf("Deployment replicas are ready: %s", deploymentName))
			break
		}

		time.Sleep(2 * time.Second)
	}
}

func (r *SleepScheduleReconciler) takeIngressCopy(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule, ingressName string) {

	// fmt.Println("Taking ingress copy")

	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	// Get the ingress to copy
	ingress := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: ingressName}, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to get ingress for copy")
		return
	}

	// Marshal the ingress to YAML
	ingressYAML, err := yaml.Marshal(ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to marshal ingress YAML")
		return
	}

	// Create the ConfigMap to store the ingress copy
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName + "-ingress-copy-" + ingressName,
			Namespace: sleepSchedule.Namespace,
		},
		Data: map[string]string{
			"ingressYAML": string(ingressYAML),
		},
	}
	ctrl.SetControllerReference(sleepSchedule, configMap, r.Scheme)

	// Try to create or update the configmap
	if err := r.Create(ctx, configMap); err != nil {
		if err := r.Update(ctx, configMap); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create or update ConfigMap")
		}
	}
}

func (r *SleepScheduleReconciler) deploySnorlaxProxy(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	// Create the snorlax service for this ingress
	snorlaxService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: sleepSchedule.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "snorlax",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	ctrl.SetControllerReference(sleepSchedule, snorlaxService, r.Scheme)

	// Check if the service already exists
	existingService := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, existingService)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing Snorlax service")
		return
	}

	// Create the service if it doesn't exist
	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, snorlaxService)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create Snorlax service")
			return
		}
	}

	// Create service account
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: sleepSchedule.Namespace,
		},
	}
	ctrl.SetControllerReference(sleepSchedule, serviceAccount, r.Scheme)

	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, serviceAccount)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing service account")
		return
	}
	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, serviceAccount)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create service account")
			return
		}
	}

	// Create role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: sleepSchedule.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "update", "patch"},
			},
		},
	}

	ctrl.SetControllerReference(sleepSchedule, role, r.Scheme)

	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, role)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing role")
		return
	}
	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, role)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create role")
			return
		}
	}
	// Create role binding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: sleepSchedule.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      objectName,
				Namespace: sleepSchedule.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     objectName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	ctrl.SetControllerReference(sleepSchedule, roleBinding, r.Scheme)

	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, roleBinding)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing role binding")
		return

	}
	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, roleBinding)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create role binding")
			return
		}
	}

	// Create the configmap for proxy data
	proxyDataConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sleep-data", objectName),
			Namespace: sleepSchedule.Namespace,
		},
		Data: map[string]string{
			"received-request": "false",
		},
	}

	ctrl.SetControllerReference(sleepSchedule, proxyDataConfigMap, r.Scheme)

	// Check if the configmap already exists
	existingConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: fmt.Sprintf("%s-sleep-data", objectName)}, existingConfigMap)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing proxy data configmap")
		return
	}

	// Create the configmap if it doesn't exist
	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, proxyDataConfigMap)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create proxy data configmap")
			return
		}
	}

	// Deploy Snorlax container and service
	snorlaxDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: sleepSchedule.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "snorlax",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "snorlax",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: objectName,
					Containers: []corev1.Container{
						{
							Name:            "snorlax",
							Image:           "ghcr.io/moonbeam-nyc/snorlax-proxy:0.4.0",
							ImagePullPolicy: "IfNotPresent",
							Env: []corev1.EnvVar{
								{
									Name:  "SNORLAX_DATA_CONFIGMAP",
									Value: fmt.Sprintf("%s-sleep-data", objectName),
								},
								{
									Name:  "SNORLAX_PORT",
									Value: "8080",
								},
								{
									Name:  "SNORLAX_NAMESPACE",
									Value: sleepSchedule.Namespace,
								},
							},
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(sleepSchedule, snorlaxDeployment, r.Scheme)

	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, existingDeployment)
	if err != nil && client.IgnoreNotFound(err) != nil {
		log.FromContext(ctx).Error(err, "Failed to get existing Snorlax deployment")
		return
	}

	if err != nil && client.IgnoreNotFound(err) == nil {
		err = r.Create(ctx, snorlaxDeployment)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create Snorlax deployment")
			return
		}
	}

	// Wait for Snorlax deployment to be ready
	time.Sleep(1 * time.Second)
	r.waitForDeploymentToWake(ctx, sleepSchedule.Namespace, objectName)

}

func (r *SleepScheduleReconciler) pointIngressToSnorlax(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule, ingressName string) {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	// Update ingress to point to snorlax service
	ingress := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: ingressName}, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Ingress for update")
		return
	}

	// Get the Ingress class from either the IngressClassName or the Ingress class annotation
	ingressClass := ""
	if ingress.Spec.IngressClassName != nil {
		ingressClass = *ingress.Spec.IngressClassName
	} else {
		ingressClass = ingress.Annotations["kubernetes.io/ingress.class"]
	}

	// Use the right path and path type based on the Ingress class
	path := "/"
	pathType := networkingv1.PathTypePrefix
	if ingressClass == "alb" {
		path = "/*"
		pathType = networkingv1.PathTypeImplementationSpecific
	}

	newRules := []networkingv1.IngressRule{}
	for _, rule := range ingress.Spec.Rules {
		newRule := networkingv1.IngressRule{
			Host: rule.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: objectName,
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		}
		newRules = append(newRules, newRule)
	}

	ingress.Spec.Rules = newRules

	err = r.Update(ctx, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Ingress to point to Snorlax")
		return
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}

func (r *SleepScheduleReconciler) loadIngressCopy(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule, ingressName string) error {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName + "-ingress-copy-" + ingressName}, configMap)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ConfigMap")
		return err
	}

	ingress := &networkingv1.Ingress{}
	err = yaml.Unmarshal([]byte(configMap.Data["ingressYAML"]), ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to unmarshal Ingress YAML")
		return err
	}

	ingressSpecJSON, err := json.Marshal(ingress.Spec)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to marshal Ingress spec into JSON")
		return err
	}

	err = r.Patch(ctx, ingress, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec": %s}`, ingressSpecJSON))))
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to patch Ingress with original spec")
	}

	return nil
}

func (r *SleepScheduleReconciler) DeleteSnorlaxProxy(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) error {
	objectName := fmt.Sprintf("snorlax-%s", sleepSchedule.Name)

	// Delete the Snorlax service
	snorlaxService := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, snorlaxService)
	if err == nil {
		err = r.Delete(ctx, snorlaxService)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to delete Snorlax service")
			return err
		}
	}

	// Delete the Snorlax deployment
	snorlaxDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, snorlaxDeployment)
	if err == nil {
		err = r.Delete(ctx, snorlaxDeployment)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to delete Snorlax deployment")
			return err
		}
	}

	// Delete the Snorlax role
	role := &rbacv1.Role{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, role)
	if err == nil {
		err = r.Delete(ctx, role)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to delete Snorlax role")
			return err
		}
	}

	// Delete the Snorlax role binding
	roleBinding := &rbacv1.RoleBinding{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, roleBinding)
	if err == nil {
		err = r.Delete(ctx, roleBinding)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to delete Snorlax role binding")
			return err
		}
	}

	// Delete the Snorlax service account
	serviceAccount := &corev1.ServiceAccount{}
	err = r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: objectName}, serviceAccount)
	if err == nil {
		err = r.Delete(ctx, serviceAccount)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to delete Snorlax service account")
			return err
		}
	}

	return nil
}

func (r *SleepScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&snorlaxv1beta1.SleepSchedule{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
