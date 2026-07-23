/*
Copyright 2026.

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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha2 "github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/internal/controller/testutils"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/config"
	"github.com/giantswarm/silence-operator/pkg/enforce"
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
				nil,
				nil,
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
				nil,
				nil,
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

	Context("Namespace enforcement", func() {
		ctx := context.Background()

		// newEnforcer writes the given config YAML to a temp file and loads it.
		newEnforcer := func(configYAML string) *enforce.Enforcer {
			path := filepath.Join(GinkgoT().TempDir(), "enforcement.yaml")
			Expect(os.WriteFile(path, []byte(configYAML), 0o600)).To(Succeed())
			enforcer, err := enforce.LoadFromFile(path)
			Expect(err).NotTo(HaveOccurred())
			return enforcer
		}

		// createNamespace creates a namespace with the given labels.
		createNamespace := func(name string, lbls map[string]string) {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		}

		// findMatcher returns the matcher with the given name, or nil.
		findMatcher := func(sil *alertmanager.Silence, name string) *alertmanager.Matcher {
			for i := range sil.Matchers {
				if sil.Matchers[i].Name == name {
					return &sil.Matchers[i]
				}
			}
			return nil
		}

		const enforcementConfig = `
matcherLabel: namespace
rules:
  - namespaceSelector:
      matchLabels:
        tenant-isolation: "enabled"
    matchers:
      - name: cluster_id
        value: prod
        matchType: "="
`

		It("injects the namespace matcher and overrides a conflicting user matcher for an enforced namespace", func() {
			nsName := "enforced-ns"
			createNamespace(nsName, map[string]string{"tenant-isolation": "enabled"})

			By("Creating a Silence with a conflicting namespace matcher")
			resource := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{Name: "enforced-silence", Namespace: nsName},
				Spec: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert"},
						// A tenant attempting to widen scope to another namespace.
						{Name: "namespace", Value: "other-namespace"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			mockServer := testutils.NewMockAlertmanagerServer()
			defer mockServer.Close()
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())

			recorder := record.NewFakeRecorder(10)
			reconciler := NewSilenceV2Reconciler(
				k8sClient,
				service.NewSilenceService(alertManager),
				tenancy.NewHelper(config.Config{}),
				newEnforcer(enforcementConfig),
				recorder,
			)

			By("Reconciling the resource")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "enforced-silence", Namespace: nsName},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the created Alertmanager silence has the enforced matchers")
			silences := mockServer.GetSilences()
			Expect(silences).To(HaveLen(1))
			created := silences[0]

			nsMatcher := findMatcher(created, "namespace")
			Expect(nsMatcher).NotTo(BeNil())
			Expect(nsMatcher.Value).To(Equal(nsName), "user-supplied namespace matcher must be overridden")
			Expect(nsMatcher.IsEqual).To(BeTrue())
			Expect(nsMatcher.IsRegex).To(BeFalse())

			clusterMatcher := findMatcher(created, "cluster_id")
			Expect(clusterMatcher).NotTo(BeNil())
			Expect(clusterMatcher.Value).To(Equal("prod"))

			Expect(findMatcher(created, "alertname")).NotTo(BeNil(), "user matchers on other labels are preserved")

			By("Verifying a MatcherOverridden event was emitted")
			Eventually(recorder.Events).Should(Receive(ContainSubstring("MatcherOverridden")))

			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("does not enforce on namespaces that do not match any rule", func() {
			nsName := "unenforced-ns"
			createNamespace(nsName, map[string]string{"tenant-isolation": "disabled"})

			resource := &observabilityv1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{Name: "unenforced-silence", Namespace: nsName},
				Spec: observabilityv1alpha2.SilenceSpec{
					Matchers: []observabilityv1alpha2.SilenceMatcher{
						{Name: "alertname", Value: "TestAlert"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			mockServer := testutils.NewMockAlertmanagerServer()
			defer mockServer.Close()
			alertManager, err := mockServer.GetAlertmanager()
			Expect(err).NotTo(HaveOccurred())

			recorder := record.NewFakeRecorder(10)
			reconciler := NewSilenceV2Reconciler(
				k8sClient,
				service.NewSilenceService(alertManager),
				tenancy.NewHelper(config.Config{}),
				newEnforcer(enforcementConfig),
				recorder,
			)

			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "unenforced-silence", Namespace: nsName},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying no enforcement matchers were injected")
			silences := mockServer.GetSilences()
			Expect(silences).To(HaveLen(1))
			created := silences[0]
			Expect(findMatcher(created, "namespace")).To(BeNil())
			Expect(findMatcher(created, "cluster_id")).To(BeNil())
			Expect(findMatcher(created, "alertname")).NotTo(BeNil())

			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})
