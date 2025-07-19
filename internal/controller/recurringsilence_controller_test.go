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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

var _ = Describe("RecurringSilence Controller", func() {
	const (
		recurringSilenceName      = "test-recurring-silence"
		recurringSilenceNamespace = "default"
	)

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      recurringSilenceName,
		Namespace: recurringSilenceNamespace,
	}

	AfterEach(func() {
		By("deleting any created Silences")
		silenceList := &v1alpha2.SilenceList{}
		err := k8sClient.List(ctx, silenceList, &client.ListOptions{Namespace: recurringSilenceNamespace})
		Expect(err).NotTo(HaveOccurred())
		for _, silence := range silenceList.Items {
			Expect(k8sClient.Delete(ctx, &silence)).To(Succeed())
		}

		By("deleting the RecurringSilence resource")
		resource := &v1alpha2.RecurringSilence{}
		err = k8sClient.Get(ctx, typeNamespacedName, resource)
		if err == nil {
			resource.Finalizers = []string{}
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, resource))
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		}
	})

	When("a RecurringSilence is created", func() {
		It("should create a Silence based on the schedule", func() {
			By("creating a new RecurringSilence resource")
			recurringSilence := &v1alpha2.RecurringSilence{
				ObjectMeta: metav1.ObjectMeta{
					Name:       recurringSilenceName,
					Namespace:  recurringSilenceNamespace,
					Finalizers: []string{RecurringSilenceFinalizerName},
				},
				Spec: v1alpha2.RecurringSilenceSpec{
					Schedule: "* * * * * *",
					SilenceTemplate: v1alpha2.SilenceSpec{
						Matchers: []v1alpha2.SilenceMatcher{
							{
								Name:  "alertname",
								Value: "TestAlert",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, recurringSilence)).To(Succeed())

			// Manually update the status to set the last schedule time in the past
			By("manually setting the last schedule time in the past")
			recurringSilence.Status.LastScheduleTime = &metav1.Time{Time: time.Now().Add(-1 * time.Minute)}
			Expect(k8sClient.Status().Update(ctx, recurringSilence)).To(Succeed())

			reconciler := &RecurringSilenceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("reconciling the RecurringSilence")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("checking that a Silence has been created")
			Eventually(func() int {
				silenceList := &v1alpha2.SilenceList{}
				err := k8sClient.List(ctx, silenceList, &client.ListOptions{Namespace: recurringSilenceNamespace})
				Expect(err).NotTo(HaveOccurred())
				return len(silenceList.Items)
			}, time.Second*5, time.Millisecond*250).Should(Equal(1))
		})
	})
})
