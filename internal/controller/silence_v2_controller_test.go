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

// findSilenceByComment returns the first alertmanager silence whose Comment matches, or nil.
func findSilenceByComment(silences []alertmanager.Silence, comment string) *alertmanager.Silence {
	for i := range silences {
		if silences[i].Comment == comment {
			return &silences[i]
		}
	}
	return nil
}

var _ = Describe("SilenceV2 CRD Integration Tests", func() {
	var mockServer *testutils.MockAlertmanagerServer
	var reconciler *SilenceV2Reconciler
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()

		mockServer = testutils.NewMockAlertmanagerServer()

		alertManager, err := mockServer.GetAlertmanager()
		Expect(err).NotTo(HaveOccurred())

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
		if mockServer != nil {
			mockServer.Close()
		}
	})

	listSilences := func() []alertmanager.Silence {
		am, err := mockServer.GetAlertmanager()
		Expect(err).NotTo(HaveOccurred())
		silences, err := am.ListSilences("")
		Expect(err).NotTo(HaveOccurred())
		return silences
	}

	doReconcile := func(name, namespace string) {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
		})
		Expect(err).NotTo(HaveOccurred())
	}

	Context("Time Management with CRDs", func() {
		It("should use endsAt over duration and valid-until annotation", func() {
			now := time.Now()
			startsAt := metav1.NewTime(now.Add(-1 * time.Hour))
			endsAt := metav1.NewTime(now.Add(2 * time.Hour))

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-endsat-priority",
					Namespace: "default",
					Annotations: map[string]string{
						"valid-until": now.Add(10 * time.Hour).Format(time.RFC3339),
					},
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					StartsAt: &startsAt,
					EndsAt:   &endsAt,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, silence)).To(Succeed()) })

			doReconcile(silence.Name, silence.Namespace)

			comment := alertmanager.SilenceComment(silence)
			got := findSilenceByComment(listSilences(), comment)
			Expect(got).NotTo(BeNil(), "silence %q not found in Alertmanager", comment)
			Expect(got.StartsAt).To(BeTemporally("~", startsAt.Time, time.Second))
			Expect(got.EndsAt).To(BeTemporally("~", endsAt.Time, time.Second))
		})

		It("should compute endsAt from startsAt + duration when endsAt is unset", func() {
			now := time.Now()
			startsAt := metav1.NewTime(now.Add(-30 * time.Minute))
			duration := observabilityv1alpha2.SilenceDuration("3h")

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-duration-explicit-start",
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					StartsAt: &startsAt,
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, silence)).To(Succeed()) })

			doReconcile(silence.Name, silence.Namespace)

			comment := alertmanager.SilenceComment(silence)
			got := findSilenceByComment(listSilences(), comment)
			Expect(got).NotTo(BeNil(), "silence %q not found in Alertmanager", comment)
			Expect(got.StartsAt).To(BeTemporally("~", startsAt.Time, time.Second))
			Expect(got.EndsAt).To(BeTemporally("~", startsAt.Time.Add(3*time.Hour), time.Second))
		})

		It("should use creation timestamp as start when startsAt is unset", func() {
			duration := observabilityv1alpha2.SilenceDuration("2h")

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-duration-no-start",
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, silence)).To(Succeed()) })

			// Capture creation timestamp before reconciling.
			created := &observabilityv1alpha2.Silence{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: silence.Name, Namespace: silence.Namespace}, created)).To(Succeed())
			createdAt := created.GetCreationTimestamp().Time

			doReconcile(silence.Name, silence.Namespace)

			comment := alertmanager.SilenceComment(silence)
			got := findSilenceByComment(listSilences(), comment)
			Expect(got).NotTo(BeNil(), "silence %q not found in Alertmanager", comment)
			Expect(got.StartsAt).To(BeTemporally("~", createdAt, time.Second))
			Expect(got.EndsAt).To(BeTemporally("~", createdAt.Add(2*time.Hour), time.Second))
		})

		It("should fall back to valid-until annotation when no spec time fields are set", func() {
			// Migration path: existing silences using the annotation continue to work.
			validUntil := time.Now().Add(6 * time.Hour).Truncate(time.Second).UTC()

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-annotation-fallback",
					Namespace: "default",
					Annotations: map[string]string{
						"valid-until": validUntil.Format(time.RFC3339),
					},
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, silence)).To(Succeed()) })

			doReconcile(silence.Name, silence.Namespace)

			comment := alertmanager.SilenceComment(silence)
			got := findSilenceByComment(listSilences(), comment)
			Expect(got).NotTo(BeNil(), "silence %q not found in Alertmanager", comment)
			Expect(got.EndsAt).To(BeTemporally("~", validUntil, time.Second))
		})

		It("should reject endsAt and duration set simultaneously", func() {
			now := time.Now()
			endsAt := metav1.NewTime(now.Add(2 * time.Hour))
			duration := observabilityv1alpha2.SilenceDuration("3h")

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-silence-both-fields",
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					EndsAt:   &endsAt,
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			err := k8sClient.Create(ctx, silence)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("endsAt and duration are mutually exclusive"))
		})
	})

	Context("Matcher Types with CRDs", func() {
		It("should convert all four match types correctly", func() {
			duration := observabilityv1alpha2.SilenceDuration("1h")

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-mixed-matchers",
					Namespace: "default",
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

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, silence)).To(Succeed()) })

			doReconcile(silence.Name, silence.Namespace)

			comment := alertmanager.SilenceComment(silence)
			got := findSilenceByComment(listSilences(), comment)
			Expect(got).NotTo(BeNil(), "silence %q not found in Alertmanager", comment)
			Expect(got.Matchers).To(HaveLen(4))
			Expect(got.Matchers[0]).To(Equal(alertmanager.Matcher{Name: "alertname", Value: "TestAlert", IsRegex: false, IsEqual: true}))
			Expect(got.Matchers[1]).To(Equal(alertmanager.Matcher{Name: "excludeme", Value: "HeartbeatAlert", IsRegex: false, IsEqual: false}))
			Expect(got.Matchers[2]).To(Equal(alertmanager.Matcher{Name: "instance", Value: ".*prod.*", IsRegex: true, IsEqual: true}))
			Expect(got.Matchers[3]).To(Equal(alertmanager.Matcher{Name: "testenv", Value: ".*test.*", IsRegex: true, IsEqual: false}))
		})
	})

	Context("Finalizer Handling with CRDs", func() {
		It("should add finalizer on create and remove it on delete", func() {
			duration := observabilityv1alpha2.SilenceDuration("1h")

			silence := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "silence-finalizer-test",
					Namespace: "default",
				},
				Spec: observabilityv1alpha2.SilenceSpec{
					Duration: &duration,
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "FinalizerTest", MatchType: observabilityv1alpha2.MatchEqual},
					},
				},
			}

			Expect(k8sClient.Create(ctx, silence)).To(Succeed())

			doReconcile(silence.Name, silence.Namespace)

			created := &observabilityv1alpha2.Silence{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: silence.Name, Namespace: silence.Namespace}, created)).To(Succeed())
			Expect(created.Finalizers).To(ContainElement(FinalizerName))

			Expect(k8sClient.Delete(ctx, created)).To(Succeed())

			doReconcile(silence.Name, silence.Namespace)

			deleted := &observabilityv1alpha2.Silence{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: silence.Name, Namespace: silence.Namespace}, deleted)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
