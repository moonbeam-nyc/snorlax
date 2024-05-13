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
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	snorlaxv1beta1 "moon-society/snorlax/api/v1beta1"
)

// SleepScheduleReconciler reconciles a SleepSchedule object
type SleepScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=snorlax.moon-society.io,resources=sleepschedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snorlax.moon-society.io,resources=sleepschedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=snorlax.moon-society.io,resources=sleepschedules/finalizers,verbs=update

func (r *SleepScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the SleepSchedule instance
	sleepSchedule := &snorlaxv1beta1.SleepSchedule{}
	err := r.Get(ctx, req.NamespacedName, sleepSchedule)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now := time.Now()

	wakeTime, err := time.Parse("15:04", sleepSchedule.Spec.WakeTime)
	if err != nil {
		log.Error(err, "failed to parse wake time")
		return ctrl.Result{}, err
	}

	sleepTime, err := time.Parse("15:04", sleepSchedule.Spec.SleepTime)
	if err != nil {
		log.Error(err, "failed to parse sleep time")
		return ctrl.Result{}, err
	}

	wakeDatetime := time.Date(now.Year(), now.Month(), now.Day(), wakeTime.Hour(), wakeTime.Minute(), 0, 0, time.Local)
	sleepDatetime := time.Date(now.Year(), now.Month(), now.Day(), sleepTime.Hour(), sleepTime.Minute(), 0, 0, time.Local)

	// Determine if the app should be awake or asleep
	var shouldSleep bool
	if wakeDatetime.Before(sleepDatetime) {
		shouldSleep = now.Before(wakeDatetime) || now.After(sleepDatetime)
	} else {
		shouldSleep = now.Before(sleepDatetime) || now.After(wakeDatetime)
	}

	awake, err := r.isAppAwake(ctx, sleepSchedule)
	if err != nil {
		log.Error(err, "Failed to determine if the application is awake")
		return ctrl.Result{}, err
	}

	if awake && shouldSleep {
		log.Info("Going to sleep")
		r.sleep(ctx, sleepSchedule)
	} else if !awake && !shouldSleep {
		log.Info("Waking up")
		r.wake(ctx, sleepSchedule)
	}

	// Update status based on the actual check
	sleepSchedule.Status.Awake = awake
	err = r.Status().Update(ctx, sleepSchedule)
	if err != nil {
		log.Error(err, "Failed to update SleepSchedule status")
		return ctrl.Result{}, err
	}

	// Requeue to check again in 10 seconds
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *SleepScheduleReconciler) isAppAwake(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) (bool, error) {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: sleepSchedule.Namespace, Name: sleepSchedule.Spec.DeploymentName}, deployment)
	if err != nil {
		return false, err
	}

	// Consider "awake" if at least one replica is available
	return deployment.Status.Replicas > 0, nil
}

func (r *SleepScheduleReconciler) wake(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) {
	r.scaleDeployment(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.DeploymentName, int32(sleepSchedule.Spec.ReplicaCount))

	if sleepSchedule.Spec.IngressName != "" {
		r.waitForDeploymentToWake(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.DeploymentName)
		r.loadIngressCopy(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.IngressName)
	}
}

func (r *SleepScheduleReconciler) sleep(ctx context.Context, sleepSchedule *snorlaxv1beta1.SleepSchedule) {
	if sleepSchedule.Spec.IngressName != "" {
		r.takeIngressCopy(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.IngressName)
		r.pointIngressToSnorlax(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.IngressName)
	}

	r.scaleDeployment(ctx, sleepSchedule.Namespace, sleepSchedule.Spec.DeploymentName, 0)
}

func (r *SleepScheduleReconciler) scaleDeployment(ctx context.Context, namespace, deploymentName string, replicaCount int32) {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Deployment")
		return
	}
	deployment.Spec.Replicas = &replicaCount
	err = r.Update(ctx, deployment)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Deployment replicas")
	}
}

func (r *SleepScheduleReconciler) waitForDeploymentToWake(ctx context.Context, namespace, deploymentName string) {
	logger := log.FromContext(ctx)

	for {
		logger.Info("Waiting for deployment to wake")
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to get Deployment")
			return
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			logger.Info("Deployment replicas are ready")
			break
		}

		time.Sleep(2 * time.Second)
	}
}

func (r *SleepScheduleReconciler) takeIngressCopy(ctx context.Context, namespace, ingressName string) {
	ingress := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ingressName}, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Ingress for copy")
		return
	}
	ingressYAML, err := yaml.Marshal(ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to marshal Ingress YAML")
		return
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snorlax.ingress." + ingressName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"ingressYAML": string(ingressYAML),
		},
	}

	if err := r.Create(ctx, configMap); err != nil {
		if err := r.Update(ctx, configMap); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create or update ConfigMap")
		}
	}
}

func (r *SleepScheduleReconciler) pointIngressToSnorlax(ctx context.Context, namespace, ingressName string) {
	ingress := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ingressName}, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Ingress for update")
		return
	}
	pathType := networkingv1.PathTypeImplementationSpecific
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "snorlax-nginx",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	}

	err = r.Update(ctx, ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Ingress to point to Snorlax")
	}
}

func (r *SleepScheduleReconciler) loadIngressCopy(ctx context.Context, namespace, ingressName string) {
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "snorlax.ingress." + ingressName}, configMap)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ConfigMap")
		return
	}

	ingress := &networkingv1.Ingress{}
	err = yaml.Unmarshal([]byte(configMap.Data["ingressYAML"]), ingress)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to unmarshal Ingress YAML")
		return
	}

	ingressSpecJSON, err := json.Marshal(ingress.Spec)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to marshal Ingress spec into JSON")
		return
	}

	err = r.Patch(ctx, ingress, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec": %s}`, ingressSpecJSON))))
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to patch Ingress with original spec")
	}
}

func (r *SleepScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&snorlaxv1beta1.SleepSchedule{}).
		Complete(r)
}
