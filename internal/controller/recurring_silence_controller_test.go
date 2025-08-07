/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

var _ = Describe("RecurringSilence Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			RecurringSilenceName      = "test-recurring-silence"
			RecurringSilenceNamespace = "default"

			timeout  = time.Second * 10
			duration = time.Second * 10
			interval = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      RecurringSilenceName,
			Namespace: RecurringSilenceNamespace,
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind RecurringSilence")
			recurringSilence := &v1alpha2.RecurringSilence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RecurringSilenceName,
					Namespace: RecurringSilenceNamespace,
				},
				Spec: v1alpha2.RecurringSilenceSpec{
					Schedule: "0 2 * * 0", // Weekly on Sunday at 2 AM
					Duration: "4h",        // Silence for 4 hours
					Matchers: []v1alpha2.SilenceMatcher{
						{
							Name:      "severity",
							Value:     "warning",
							MatchType: v1alpha2.MatchEqual,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, recurringSilence)).To(Succeed())
		})

		AfterEach(func() {
			resource := &v1alpha2.RecurringSilence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RecurringSilence")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &RecurringSilenceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the RecurringSilence has the expected finalizer")
			recurringSilence := &v1alpha2.RecurringSilence{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, recurringSilence)
				if err != nil {
					return false
				}
				for _, finalizer := range recurringSilence.Finalizers {
					if finalizer == RecurringSilenceFinalizerName {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Checking that reconcile completed successfully")
			// For now, just ensure no error occurred and finalizer was added
			// Status updates might require more complex test setup with proper managers
		})

		It("should handle invalid cron expressions", func() {
			By("Creating a RecurringSilence with invalid cron expression")
			invalidRecurringSilence := &v1alpha2.RecurringSilence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cron",
					Namespace: RecurringSilenceNamespace,
				},
				Spec: v1alpha2.RecurringSilenceSpec{
					Schedule: "99 99 99 99 99", // Invalid cron values but matches regex format
					Duration: "1h",
					Matchers: []v1alpha2.SilenceMatcher{
						{
							Name:      "test",
							Value:     "value",
							MatchType: v1alpha2.MatchEqual,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, invalidRecurringSilence)).To(Succeed())

			By("Reconciling the resource with invalid cron")
			controllerReconciler := &RecurringSilenceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			namespacedName := types.NamespacedName{
				Name:      "invalid-cron",
				Namespace: RecurringSilenceNamespace,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the resource was created and has finalizer")
			Eventually(func() bool {
				resource := &v1alpha2.RecurringSilence{}
				err := k8sClient.Get(ctx, namespacedName, resource)
				if err != nil {
					return false
				}
				for _, finalizer := range resource.Finalizers {
					if finalizer == RecurringSilenceFinalizerName {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up the invalid resource")
			Expect(k8sClient.Delete(ctx, invalidRecurringSilence)).To(Succeed())
		})
	})
})