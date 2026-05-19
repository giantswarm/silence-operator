//go:build e2e

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

var _ = Describe("Multiple matchers", func() {
	const (
		namespace   = "test-multi-match"
		silenceName = "test-multi"
	)

	ctx := context.Background()
	expectedComment := "silence-operator-" + namespace + "-" + silenceName

	BeforeEach(func() {
		Expect(createNamespace(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
		}
		_ = client.IgnoreNotFound(k8sClient.Delete(ctx, silence))
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence)
			return err != nil
		}, "60s", "2s").Should(BeTrue())
	})

	It("should create an AM silence with all matchers", func() {
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{
						Name:      "alertname",
						Value:     "HighLatency",
						MatchType: v1alpha2.MatchEqual,
					},
					{
						Name:      "namespace",
						Value:     "kube-system",
						MatchType: v1alpha2.MatchEqual,
					},
					{
						Name:      "severity",
						Value:     "warning|critical",
						MatchType: v1alpha2.MatchRegexMatch,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())

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

		Expect(amSilence.Matchers).To(HaveLen(3))

		matchersByName := make(map[string]alertmanager.Matcher)
		for _, m := range amSilence.Matchers {
			matchersByName[m.Name] = m
		}

		Expect(matchersByName).To(HaveKey("alertname"))
		Expect(matchersByName["alertname"].Value).To(Equal("HighLatency"))
		Expect(matchersByName["alertname"].IsRegex).To(BeFalse())

		Expect(matchersByName).To(HaveKey("namespace"))
		Expect(matchersByName["namespace"].Value).To(Equal("kube-system"))

		Expect(matchersByName).To(HaveKey("severity"))
		Expect(matchersByName["severity"].Value).To(Equal("warning|critical"))
		Expect(matchersByName["severity"].IsRegex).To(BeTrue())
	})
})
