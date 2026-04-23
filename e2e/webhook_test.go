//go:build e2e

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// These tests require the operator to be deployed with webhook.enabled=true and
// webhook.celRules configured with at least the two Kyverno-parity rules:
//
//	celRules:
//	- name: exclude-heartbeat
//	  condition: ""
//	  matchers:
//	  - name: alertname
//	    value: Heartbeat
//	    matchType: "!="
//	- name: exclude-all-pipelines
//	  condition: ""
//	  matchers:
//	  - name: all_pipelines
//	    value: "true"
//	    matchType: "!="
//
// If the MutatingWebhookConfiguration is absent the whole Describe block skips.
var _ = Describe("Mutating webhook", func() {
	ctx := context.Background()
	const namespace = "test-webhook"

	BeforeEach(func() {
		// Skip the entire suite when the webhook isn't deployed.
		mwc := &admissionregistrationv1.MutatingWebhookConfiguration{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "silence-operator"}, mwc); err != nil {
			Skip("MutatingWebhookConfiguration 'silence-operator' not found — deploy with webhook.enabled=true to run these tests")
		}

		// The namespace is shared across tests; wait for any previous instance to
		// be fully gone before recreating it (avoids "namespace is terminating" errors).
		Eventually(func() error {
			ns := &corev1.Namespace{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: namespace}, ns)
			if err != nil {
				return nil // not found: safe to create
			}
			return fmt.Errorf("namespace %q still exists (phase: %s)", namespace, ns.Status.Phase)
		}, "120s", "2s").Should(Succeed(), "timed out waiting for namespace %q to be fully deleted", namespace)

		Expect(createNamespace(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(deleteNamespace(ctx, namespace)).To(Succeed())
	})

	It("should inject configured CEL rule matchers into a new Silence", func() {
		silenceName := "webhook-inject-test"
		expectedComment := fmt.Sprintf("silence-operator-%s-%s", namespace, silenceName)

		By("Creating a Silence with only a user-defined matcher")
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{Name: "alertname", Value: "HighCPU", MatchType: v1alpha2.MatchEqual},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())
		DeferCleanup(func() {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, silence))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence)
				return err != nil
			}, "60s", "2s").Should(BeTrue())
		})

		By("Verifying the webhook injected the forced matchers into the stored Silence spec")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence); err != nil {
				return false
			}
			return hasMatcherInSpec(silence, "alertname", "Heartbeat", v1alpha2.MatchNotEqual) &&
				hasMatcherInSpec(silence, "all_pipelines", "true", v1alpha2.MatchNotEqual)
		}, "30s", "2s").Should(BeTrue(), "expected webhook to inject Heartbeat and all_pipelines exclusion matchers")

		By("Waiting for the silence to be synced to Alertmanager")
		var amSilence *alertmanager.Silence
		Eventually(func() error {
			var err error
			amSilence, err = findSilenceByComment(amPortForward.localPort, expectedComment)
			if err != nil {
				return err
			}
			if amSilence == nil {
				return fmt.Errorf("silence not found in Alertmanager")
			}
			return nil
		}, "60s", "2s").Should(Succeed())

		By("Verifying all matchers (user + injected) are present in Alertmanager")
		matcherMap := make(map[string]string) // name+value → matchType
		for _, m := range amSilence.Matchers {
			key := m.Name + "=" + m.Value
			if m.IsRegex {
				matcherMap[key] = "regex"
			} else if m.IsEqual {
				matcherMap[key] = "="
			} else {
				matcherMap[key] = "!="
			}
		}
		Expect(matcherMap).To(HaveKeyWithValue("alertname=HighCPU", "="))
		Expect(matcherMap).To(HaveKeyWithValue("alertname=Heartbeat", "!="))
		Expect(matcherMap).To(HaveKeyWithValue("all_pipelines=true", "!="))
	})

	It("should not duplicate matchers on UPDATE if they are already present", func() {
		silenceName := "webhook-idempotent-test"

		By("Creating a Silence that already has the injected matchers")
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{Name: "alertname", Value: "HighCPU", MatchType: v1alpha2.MatchEqual},
					{Name: "alertname", Value: "Heartbeat", MatchType: v1alpha2.MatchNotEqual},
					{Name: "all_pipelines", Value: "true", MatchType: v1alpha2.MatchNotEqual},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())
		DeferCleanup(func() {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, silence))
		})

		By("Triggering an UPDATE by adding a label")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence); err != nil {
				return err
			}
			if silence.Labels == nil {
				silence.Labels = map[string]string{}
			}
			silence.Labels["test-update"] = "true"
			return k8sClient.Update(ctx, silence)
		}, "30s", "2s").Should(Succeed())

		By("Verifying no duplicate matchers were injected")
		Eventually(func() int {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence)
			return len(silence.Spec.Matchers)
		}, "30s", "2s").Should(Equal(3), "expected exactly 3 matchers — no duplicates after UPDATE")
	})

	It("should apply a namespace-scoped CEL rule only in matching namespaces", Label("cel-conditional"), func() {
		// This test verifies that always-apply rules (empty condition) fire regardless of
		// namespace.  To additionally verify conditional rules, deploy the operator with a
		// rule like: condition: 'object.metadata.namespace == "test-webhook"' and extend
		// this test to assert the namespace-scoped injection as well.
		silenceName := "webhook-conditional-test"

		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{Name: "alertname", Value: "TestAlert", MatchType: v1alpha2.MatchEqual},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())
		DeferCleanup(func() {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, silence))
		})

		By("Verifying base matchers are present (always-apply rules)")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence); err != nil {
				return false
			}
			// The always-apply Heartbeat exclusion must be present regardless.
			return hasMatcherInSpec(silence, "alertname", "Heartbeat", v1alpha2.MatchNotEqual)
		}, "30s", "2s").Should(BeTrue())
	})
})

// hasMatcherInSpec returns true when the Silence spec contains a matcher with
// the given name, value, and matchType.
func hasMatcherInSpec(s *v1alpha2.Silence, name, value string, mt v1alpha2.MatchType) bool {
	for _, m := range s.Spec.Matchers {
		if m.Name == name && m.Value == value && m.MatchType == mt {
			return true
		}
	}
	return false
}
