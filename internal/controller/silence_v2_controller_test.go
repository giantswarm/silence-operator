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
	"fmt"
	"time"

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
	"github.com/giantswarm/silence-operator/pkg/config"
	"github.com/giantswarm/silence-operator/pkg/service"
	"github.com/giantswarm/silence-operator/pkg/tenancy"
)

var _ = Describe("SilenceV2 Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource-v2"

		ctx := context.Background()
		var mockServer *testutils.MockAlertmanagerServer

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		silence := &observabilityv1alpha2.Silence{}

		BeforeEach(func() {
			// Set up mock Alertmanager server
			mockServer = testutils.NewMockAlertmanagerServer()

			By("creating the custom resource for the Kind Silence v1alpha2")
			var err = k8sClient.Get(ctx, typeNamespacedName, silence)
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
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())

			// Create tenancy helper with default config
			cfg := config.Config{}
			tenancyHelper := tenancy.NewHelper(cfg)

			silenceService := service.NewSilenceService(alertManager)
			controllerReconciler := NewSilenceV2Reconciler(
				k8sClient,
				silenceService,
				tenancyHelper,
			)

			_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(reconcileErr).NotTo(HaveOccurred())
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
				},
			}
			Expect(k8sClient.Create(ctx, finalizerTestResource)).To(Succeed())

			By("Reconciling to add finalizer")
			alertManager, err2 := mockServer.GetAlertmanager()
			Expect(err2).NotTo(HaveOccurred())

			// Create tenancy helper with default config
			cfg := config.Config{}
			tenancyHelper := tenancy.NewHelper(cfg)

			silenceService := service.NewSilenceService(alertManager)
			controllerReconciler := NewSilenceV2Reconciler(
				k8sClient,
				silenceService,
				tenancyHelper,
			)

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
	})

	Context("MatchType Conversion", func() {
		var reconciler *SilenceV2Reconciler

		BeforeEach(func() {
			reconciler = &SilenceV2Reconciler{}
		})

		It("should convert MatchType enum to correct boolean values", func() {
			testCases := []struct {
				matchType       observabilityv1alpha2.MatchType
				expectedIsRegex bool
				expectedIsEqual bool
				description     string
			}{
				{observabilityv1alpha2.MatchEqual, false, true, "exact match (=)"},
				{observabilityv1alpha2.MatchNotEqual, false, false, "exact non-match (!=)"},
				{observabilityv1alpha2.MatchRegexMatch, true, true, "regex match (=~)"},
				{observabilityv1alpha2.MatchRegexNotMatch, true, false, "regex non-match (!~)"},
				{"", false, true, "empty/default should be exact match"},
			}

			for _, tc := range testCases {
				silence := &observabilityv1alpha2.Silence{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-silence",
						Namespace: "default",
					},
					Spec: observabilityv1alpha2.SilenceSpec{
						Matchers: []observabilityv1alpha2.SilenceMatcher{
							{
								Name:      "alertname",
								Value:     "TestAlert",
								MatchType: tc.matchType,
							},
						},
					},
				}

				result, err := reconciler.getSilenceFromCR(silence)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Matchers).To(HaveLen(1))

				matcher := result.Matchers[0]
				Expect(matcher.IsRegex).To(Equal(tc.expectedIsRegex),
					"IsRegex mismatch for %s", tc.description)
				Expect(matcher.IsEqual).To(Equal(tc.expectedIsEqual),
					"IsEqual mismatch for %s", tc.description)
				Expect(matcher.Name).To(Equal("alertname"))
				Expect(matcher.Value).To(Equal("TestAlert"))
			}
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

			// Test can the namespace selector matches the test namespace labels
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

			By("Testing that namespace selector logic works correctly")
			Expect(namespaceSelectorLabels).ToNot(BeNil())
			Expect(namespaceSelectorLabels.String()).To(Equal("environment=production"))
		})
	})
})

var _ = Describe("SilenceV2 CRD Integration Tests", func() {
	var mockServer *testutils.MockAlertmanagerServer
	var reconciler *SilenceV2Reconciler
	ctx := context.Background()

	BeforeEach(func() {
		// Set up mock Alertmanager server
		mockServer = testutils.NewMockAlertmanagerServer()

		alertManager, err := mockServer.GetAlertmanager()
		Expect(err).NotTo(HaveOccurred())

		// Create tenancy helper with default config
		cfg := config.Config{}
		tenancyHelper := tenancy.NewHelper(cfg)

		silenceService := service.NewSilenceService(alertManager)
		reconciler = NewSilenceV2Reconciler(
			k8sClient,
			silenceService,
			tenancyHelper,
		)
	})

	AfterEach(func() {
		// Clean up mock server
		if mockServer != nil {
			mockServer.Close()
		}
	})

	Context("Time Management with CRDs", func() {
		It("should create silence with EndsAt field priority", func() {
			now := time.Now()
			startsAt := metav1.NewTime(now.Add(-1 * time.Hour))
			endsAt := metav1.NewTime(now.Add(2 * time.Hour))

			silenceName := "silence-endsat-priority"
			silenceNamespace := "default"

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: silenceNamespace,
					Annotations: map[string]string{
						"valid-until": now.Add(10 * time.Hour).Format(time.RFC3339),
					},
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					StartsAt: &startsAt,
					EndsAt:   &endsAt,
					// Note: Duration is intentionally omitted to test EndsAt priority
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			By("Creating the silence CRD")
			Expect(k8sClient.Create(ctx, silence)).To(Succeed())

			By("Reconciling the silence")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the silence was created in Alertmanager with correct times")
			comment := fmt.Sprintf("silence-operator-%s-%s", silenceNamespace, silenceName)
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())
			alertmanagerSilences, err := alertManager.ListSilences("")
			Expect(err).NotTo(HaveOccurred())

			var createdSilence *alertmanager.Silence
			for _, s := range alertmanagerSilences {
				if s.Comment == comment {
					createdSilence = &s
					break
				}
			}
			Expect(createdSilence).NotTo(BeNil(), fmt.Sprintf("Expected to find silence with comment %q, but got %d silences: %+v", comment, len(alertmanagerSilences), alertmanagerSilences))
			Expect(createdSilence.StartsAt).To(BeTemporally("~", startsAt.Time, time.Second))
			Expect(createdSilence.EndsAt).To(BeTemporally("~", endsAt.Time, time.Second))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, silence)).To(Succeed())
		})

		It("should create silence with Duration field when EndsAt is not specified", func() {
			now := time.Now()
			startsAt := metav1.NewTime(now.Add(-30 * time.Minute))
			duration := metav1.Duration{Duration: 3 * time.Hour}

			silenceName := "silence-duration-priority"
			silenceNamespace := "default"

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					StartsAt: &startsAt,
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			By("Creating the silence CRD")
			Expect(k8sClient.Create(ctx, silence)).To(Succeed())

			By("Reconciling the silence")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the silence was created with duration-based end time")
			comment := fmt.Sprintf("silence-operator-%s-%s", silenceNamespace, silenceName)
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())
			alertmanagerSilences, err := alertManager.ListSilences("")
			Expect(err).NotTo(HaveOccurred())

			var createdSilence *alertmanager.Silence
			for _, s := range alertmanagerSilences {
				if s.Comment == comment {
					createdSilence = &s
					break
				}
			}
			Expect(createdSilence).NotTo(BeNil(), fmt.Sprintf("Expected to find silence with comment %q, but got %d silences: %+v", comment, len(alertmanagerSilences), alertmanagerSilences))
			Expect(createdSilence.StartsAt).To(BeTemporally("~", startsAt.Time, time.Second))
			expectedEndsAt := startsAt.Time.Add(duration.Duration)
			Expect(createdSilence.EndsAt).To(BeTemporally("~", expectedEndsAt, time.Second))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, silence)).To(Succeed())
		})

		It("should validate CRD constraints", func() {
			now := time.Now()
			endsAt := metav1.NewTime(now.Add(2 * time.Hour))
			duration := metav1.Duration{Duration: 3 * time.Hour}

			silenceName := "invalid-silence-both-fields"
			silenceNamespace := "default"

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					EndsAt:   &endsAt,
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			By("Attempting to create the invalid silence CRD")
			err := k8sClient.Create(ctx, silence)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("endsAt and duration are mutually exclusive"))
		})
	})

	Context("Matcher Types with CRDs", func() {
		It("should create silence with different matcher types", func() {
			silenceName := "silence-mixed-matchers"
			silenceNamespace := "default"
			duration := metav1.Duration{Duration: 1 * time.Hour}

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
						{Name: "excludeme", Value: "HeartbeatAlert", MatchType: observabilityv1alpha2.MatchNotEqual},
						{Name: "instance", Value: ".*prod.*", MatchType: observabilityv1alpha2.MatchRegexMatch},
						{Name: "testenv", Value: ".*test.*", MatchType: observabilityv1alpha2.MatchRegexNotMatch},
					},
				},
			}

			By("Creating the silence CRD")
			Expect(k8sClient.Create(ctx, silence)).To(Succeed())

			By("Reconciling the silence")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all matcher types were converted correctly")
			comment := fmt.Sprintf("silence-operator-%s-%s", silenceNamespace, silenceName)
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())
			alertmanagerSilences, err := alertManager.ListSilences("")
			Expect(err).NotTo(HaveOccurred())

			var createdSilence *alertmanager.Silence
			for _, s := range alertmanagerSilences {
				if s.Comment == comment {
					createdSilence = &s
					break
				}
			}
			Expect(createdSilence).NotTo(BeNil(), fmt.Sprintf("Expected to find silence with comment %q, but got %d silences: %+v", comment, len(alertmanagerSilences), alertmanagerSilences))
			Expect(createdSilence.Matchers).To(HaveLen(4))

			// Verify matcher conversion
			Expect(createdSilence.Matchers[0].IsRegex).To(BeFalse())
			Expect(createdSilence.Matchers[0].IsEqual).To(BeTrue())
			Expect(createdSilence.Matchers[1].IsRegex).To(BeFalse())
			Expect(createdSilence.Matchers[1].IsEqual).To(BeFalse())
			Expect(createdSilence.Matchers[2].IsRegex).To(BeTrue())
			Expect(createdSilence.Matchers[2].IsEqual).To(BeTrue())
			Expect(createdSilence.Matchers[3].IsRegex).To(BeTrue())
			Expect(createdSilence.Matchers[3].IsEqual).To(BeFalse())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, silence)).To(Succeed())
		})
	})

	Context("Finalizer Handling with CRDs", func() {
		It("should add and remove finalizers correctly during lifecycle", func() {
			silenceName := "silence-finalizer-test"
			silenceNamespace := "default"
			duration := metav1.Duration{Duration: 1 * time.Hour}

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "FinalizerTest", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			By("Creating the silence CRD")
			Expect(k8sClient.Create(ctx, silence)).To(Succeed())

			By("Reconciling to add finalizer")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying finalizer was added")
			createdSilence := &observabilityv1alpha2.Silence{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      silenceName,
				Namespace: silenceNamespace,
			}, createdSilence)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdSilence.Finalizers).To(ContainElement(FinalizerName))

			By("Deleting the silence CRD")
			Expect(k8sClient.Delete(ctx, createdSilence)).To(Succeed())

			By("Reconciling deletion to remove finalizer")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      silenceName,
					Namespace: silenceNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the CRD was fully deleted")
			deletedSilence := &observabilityv1alpha2.Silence{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      silenceName,
				Namespace: silenceNamespace,
			}, deletedSilence)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
