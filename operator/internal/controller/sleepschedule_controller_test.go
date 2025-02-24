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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	snorlaxv1beta1 "moonbeam-nyc/snorlax/api/v1beta1"
	util "moonbeam-nyc/snorlax/internal/util"
)

var _ = Describe("SleepSchedule Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		sleepschedule := &snorlaxv1beta1.SleepSchedule{}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind SleepSchedule")
			err := k8sClient.Get(ctx, typeNamespacedName, sleepschedule)
			if err != nil && errors.IsNotFound(err) {
				resource := &snorlaxv1beta1.SleepSchedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// NOTE: The kubebuilder validations are ignored during testing, but the controller
					// logic requires exactly one of dailyWindow or cronSchedule.
					Spec: snorlaxv1beta1.SleepScheduleSpec{
						DailyWindow: &snorlaxv1beta1.DailyWindow{
							WakeTime:  "10:00am",
							SleepTime: "10:01am",
						},
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &snorlaxv1beta1.SleepSchedule{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SleepSchedule")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			// Set the current time to 12:00am
			location, err := time.LoadLocation(sleepschedule.Spec.Timezone)
			Expect(err).NotTo(HaveOccurred())
			timeStub := util.MockTime{
				Time: time.Date(2025, 1, 1, 0, 0, 0, 0, location),
			}

			// Create the reconciler
			controllerReconciler := NewReconciler(
				k8sClient,
				k8sClient.Scheme(),
				timeStub,
			)

			By("Reconciling the created resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(sleepschedule.Status.Awake).To(Equal(false))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})

		Context("with DailyWindow", func() {
			BeforeEach(func() {
				By("Setting default DailyWindow to 8:00am wake and 10:00pm sleep")
				sleepschedule.Spec = snorlaxv1beta1.SleepScheduleSpec{
					DailyWindow: &snorlaxv1beta1.DailyWindow{
						WakeTime:  "8:00am",
						SleepTime: "10:00pm",
					},
				}
				Expect(k8sClient.Update(ctx, sleepschedule)).To(Succeed())
			})

			It("should process DailyWindow spec correctly", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				// Create a time stub
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 0, 0, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule with DailyWindow")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduleData).NotTo(BeNil())
				Expect(scheduleData.DailyWindow).NotTo(BeNil())
				Expect(scheduleData.CronSchedule).To(BeNil())

				By("Checking that the wake time is correct")
				Expect(scheduleData.DailyWindow.WakeTime).To(Equal(time.Date(2025, 1, 1, 8, 0, 0, 0, location)))

				By("Checking that the sleep time is correct")
				Expect(scheduleData.DailyWindow.SleepTime).To(Equal(time.Date(2025, 1, 1, 22, 0, 0, 0, location)))
			})

			It("should determine schedule is awake at 8:01am on a weekday", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Setting the current time to 8:01am on a Wednesday")
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 8, 1, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule with DailyWindow")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Checking that the SleepSchedule should be awake")
				shouldBeAsleep, err := reconciler.shouldSleep(scheduleData)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldBeAsleep).To(BeFalse())
			})

			It("should determine schedule is asleep at 10:01pm on a weekday", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Setting the current time to 10:01pm on a Wednesday")
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 22, 1, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule with DailyWindow")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Checking that the SleepSchedule should be asleep")
				shouldBeAsleep, err := reconciler.shouldSleep(scheduleData)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldBeAsleep).To(BeTrue())
			})

		})

		Context("with CronSchedule", func() {
			BeforeEach(func() {
				By("Setting CronSchedule to wake at 8am on weekdays and sleep at 10pm daily")
				sleepschedule.Spec = snorlaxv1beta1.SleepScheduleSpec{
					CronSchedule: &snorlaxv1beta1.CronSchedule{
						WakeSchedule:  "0 8 * * 1-5", // 8 AM every weekday
						SleepSchedule: "0 22 * * *",  // 10 PM every day
					},
				}
				Expect(k8sClient.Update(ctx, sleepschedule)).To(Succeed())
			})

			It("should process CronSchedule spec correctly", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				// Create a time stub
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 0, 0, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduleData).NotTo(BeNil())
				Expect(scheduleData.CronSchedule).NotTo(BeNil())
				Expect(scheduleData.DailyWindow).To(BeNil())

				By("Checking that the wake schedule is correct")
				Expect(scheduleData.CronSchedule.WakeSchedule).To(Equal("0 8 * * 1-5"))

				By("Checking that the sleep schedule is correct")
				Expect(scheduleData.CronSchedule.SleepSchedule).To(Equal("0 22 * * *"))
			})

			It("should determine schedule is awake at 8:01am on a weekday", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Setting the current time to 8:01am on a Wednesday")
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 8, 1, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Checking that the SleepSchedule should be awake")
				shouldBeAsleep, err := reconciler.shouldSleep(scheduleData)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldBeAsleep).To(BeFalse())
			})

			It("should determine schedule is asleep at 10:01pm on a weekday", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Setting the current time to 10:01pm on a Wednesday")
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 1, 22, 1, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Checking that the SleepSchedule should be asleep")
				shouldBeAsleep, err := reconciler.shouldSleep(scheduleData)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldBeAsleep).To(BeTrue())
			})

			It("should determine schedule is asleep at 8:01am on the weekend", func() {
				// Get the existing SleepSchedule created by BeforeEach
				existingSchedule := &snorlaxv1beta1.SleepSchedule{}
				err := k8sClient.Get(ctx, typeNamespacedName, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Setting the current time to 8:01am on a Saturday")
				location, err := time.LoadLocation(existingSchedule.Spec.Timezone)
				Expect(err).NotTo(HaveOccurred())
				timeStub := util.MockTime{
					Time: time.Date(2025, 1, 4, 8, 1, 0, 0, location),
				}

				// Create reconciler
				reconciler := NewReconciler(
					k8sClient,
					k8sClient.Scheme(),
					timeStub,
				)

				By("Processing the SleepSchedule")
				scheduleData, err := reconciler.ProcessSleepSchedule(ctx, existingSchedule)
				Expect(err).NotTo(HaveOccurred())

				By("Checking that the SleepSchedule should be asleep")
				shouldBeAsleep, err := reconciler.shouldSleep(scheduleData)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldBeAsleep).To(BeTrue())
			})
		})
	})
})
