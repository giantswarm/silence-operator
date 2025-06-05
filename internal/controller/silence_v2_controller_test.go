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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha2 "github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/internal/controller/testutils"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

var _ = Describe("SilenceV2 Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource-v2"

		ctx := context.Background()
		var mockServer *testutils.MockAlertManagerServer
		var mockAlertManager *alertmanager.AlertManager

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		silence := &observabilityv1alpha2.Silence{}

		BeforeEach(func() {
			// Set up mock AlertManager server
			mockServer = testutils.NewMockAlertManagerServer()
			var err error
			mockAlertManager, err = mockServer.GetAlertManager()
			Expect(err).NotTo(HaveOccurred())

			By("creating the custom resource for the Kind Silence v1alpha2")
			err = k8sClient.Get(ctx, typeNamespacedName, silence)
			if err != nil && errors.IsNotFound(err) {
				resource := &observabilityv1alpha2.Silence{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: observabilityv1alpha2.SilenceSpec{
						Matchers: []observabilityv1alpha2.SilenceMatcher{
							{
								Name:  "alertname",
								Value: "TestAlertV2",
							},
						},
						Owner:    "test-owner-v2",
						IssueURL: "https://github.com/example/test-issue-v2",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Clean up mock server
			if mockServer != nil {
				mockServer.Close()
			}

			// Cleanup logic after each test, like removing the resource instance.
			resource := &observabilityv1alpha2.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Silence v1alpha2")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SilenceV2Reconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				Alertmanager: mockAlertManager,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})

		It("should handle deletion with finalizer", func() {
			finalizerTestResourceName := "finalizer-test-resource"
			finalizerTestNamespacedName := types.NamespacedName{
				Name:      finalizerTestResourceName,
				Namespace: "default",
			}

			By("Creating a separate resource for finalizer testing")
			finalizerTestResource := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      finalizerTestResourceName,
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{
							Name:  "alertname",
							Value: "FinalizerTestAlert",
						},
					},
					Owner:    "finalizer-test-owner",
					IssueURL: "https://github.com/example/finalizer-test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, finalizerTestResource)).To(Succeed())

			By("Reconciling to add finalizer")
			controllerReconciler := &SilenceV2Reconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				Alertmanager: mockAlertManager,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: finalizerTestNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying finalizer was added")
			createdSilence := &observabilityv1alpha2.Silence{}
			err = k8sClient.Get(ctx, finalizerTestNamespacedName, createdSilence)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdSilence.Finalizers).To(ContainElement(FinalizerName))

			By("Deleting the resource")
			Expect(k8sClient.Delete(ctx, createdSilence)).To(Succeed())

			By("Reconciling deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: finalizerTestNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying resource is deleted")
			err = k8sClient.Get(ctx, finalizerTestNamespacedName, createdSilence)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should convert v1alpha2 to v1alpha1 format correctly", func() {
			By("Creating a v1alpha2 silence")
			v2Silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conversion-test",
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: true,
						},
						{
							Name:  "severity",
							Value: "critical",
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/test/issue",
				},
			}

			By("Converting to v1alpha1 format")
			v1Silence := convertToV1Alpha1(v2Silence)

			By("Verifying conversion")
			Expect(v1Silence.ObjectMeta.Name).To(Equal("conversion-test"))
			Expect(v1Silence.ObjectMeta.Namespace).To(Equal("default"))
			Expect(v1Silence.Spec.Matchers).To(HaveLen(2))
			Expect(v1Silence.Spec.Matchers[0].Name).To(Equal("alertname"))
			Expect(v1Silence.Spec.Matchers[0].Value).To(Equal("TestAlert"))
			Expect(v1Silence.Spec.Matchers[0].IsRegex).To(BeTrue())
			Expect(v1Silence.Spec.Matchers[1].Name).To(Equal("severity"))
			Expect(v1Silence.Spec.Matchers[1].Value).To(Equal("critical"))
			Expect(v1Silence.Spec.Owner).To(Equal("test-owner"))
			Expect(v1Silence.Spec.IssueURL).To(Equal("https://github.com/test/issue"))
		})

		It("should respect namespace selector when configured", func() {
			By("Creating a namespace with specific labels")
			testNamespace := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
					Labels: map[string]string{
						"environment": "production",
						"team":        "platform",
					},
				},
			}
			testNamespace.SetGroupVersionKind(metav1.SchemeGroupVersion.WithKind("Namespace"))

			// Note: In the test environment, we can't create actual namespaces,
			// so we'll test the namespace selector logic without actual namespace creation

			By("Verifying namespace selector predicate works by testing label matching")
			namespaceSelector, err := metav1.ParseToLabelSelector("environment=production")
			Expect(err).NotTo(HaveOccurred())
			namespaceSelectorLabels, err := metav1.LabelSelectorAsSelector(namespaceSelector)
			Expect(err).NotTo(HaveOccurred())

			// Test that the namespace selector matches the test namespace labels
			Expect(namespaceSelectorLabels.Matches(labels.Set{
				"environment": "production",
				"team":        "platform",
			})).To(BeTrue())

			// Test that the namespace selector doesn't match different labels
			nonMatchingNamespaceSelector, err := metav1.ParseToLabelSelector("environment=staging")
			Expect(err).NotTo(HaveOccurred())
			nonMatchingNamespaceSelectorLabels, err := metav1.LabelSelectorAsSelector(nonMatchingNamespaceSelector)
			Expect(err).NotTo(HaveOccurred())

			Expect(nonMatchingNamespaceSelectorLabels.Matches(labels.Set{
				"environment": "production",
				"team":        "platform",
			})).To(BeFalse())

			By("Testing that controller with namespace selector can be created")
			controllerReconciler := &SilenceV2Reconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				Alertmanager:      mockAlertManager,
				NamespaceSelector: namespaceSelectorLabels,
			}

			// Note: We can't easily test the actual filtering behavior in unit tests
			// since it would require creating actual namespaces in the test environment.
			// This test verifies the configuration is properly set up.
			Expect(controllerReconciler.NamespaceSelector).ToNot(BeNil())
			Expect(controllerReconciler.NamespaceSelector.String()).To(Equal("environment=production"))
		})
	})
})
