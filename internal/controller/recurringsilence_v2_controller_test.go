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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha2 "github.com/giantswarm/silence-operator/api/v1alpha2"
)

var _ = Describe("RecurringSilence Controller", func() {
	const resourceName = "test-recurring-silence"

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: "default",
	}

	AfterEach(func() {
		By("deleting the RecurringSilence resource")
		resource := &observabilityv1alpha2.RecurringSilence{}
		err := k8sClient.Get(ctx, typeNamespacedName, resource)
		if err == nil {
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		}

		By("deleting the Silence resource")
		silence := &observabilityv1alpha2.Silence{}
		err = k8sClient.Get(ctx, typeNamespacedName, silence)
		if err == nil {
			Expect(k8sClient.Delete(ctx, silence)).To(Succeed())
		}
	})

	It("Should create a Silence from a RecurringSilence", func() {
		By("creating a new RecurringSilence resource")
		recurringSilence := &observabilityv1alpha2.RecurringSilence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
			},
			Spec: observabilityv1alpha2.RecurringSilenceSpec{
				Schedule: "* * * * *",
				Template: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{
							Name:  "alertname",
							Value: "TestAlert",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, recurringSilence)).To(Succeed())

		reconciler := &RecurringSilenceV2Reconciler{
			Client: k8sClient,
		}

		By("reconciling the created resource")
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: typeNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(And(BeNumerically(">", 0), BeNumerically("<=", time.Minute)))

		By("checking the created Silence")
		silence := &observabilityv1alpha2.Silence{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, typeNamespacedName, silence)
			return err == nil
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		Expect(silence.Spec.Matchers[0].Name).To(Equal("alertname"))
		Expect(silence.Spec.Matchers[0].Value).To(Equal("TestAlert"))
	})

	It("Should not create a Silence from a RecurringSilence with invalid schedule", func() {
		By("creating a new RecurringSilence resource with invalid schedule")
		recurringSilence := &observabilityv1alpha2.RecurringSilence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
			},
			Spec: observabilityv1alpha2.RecurringSilenceSpec{
				Schedule: "invalid schedule",
				Template: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{
							Name:  "alertname",
							Value: "TestAlert",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, recurringSilence)).To(Succeed())

		reconciler := &RecurringSilenceV2Reconciler{
			Client: k8sClient,
		}

		By("reconciling the created resource")
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: typeNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())

		By("checking that no Silence was created")
		silence := &observabilityv1alpha2.Silence{}
		Consistently(func() bool {
			err := k8sClient.Get(ctx, typeNamespacedName, silence)
			return errors.IsNotFound(err)
		}, time.Second*5, time.Millisecond*250).Should(BeTrue())
	})
})
