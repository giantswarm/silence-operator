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

var _ = Describe("v1alpha2 Silence match types", func() {
	ctx := context.Background()

	type matchTypeTestCase struct {
		matchType       v1alpha2.MatchType
		expectedIsRegex bool
		expectedIsEqual bool
	}

	entries := []TableEntry{
		Entry("MatchEqual (=)", matchTypeTestCase{
			matchType:       v1alpha2.MatchEqual,
			expectedIsRegex: false,
			expectedIsEqual: true,
		}),
		Entry("MatchNotEqual (!=)", matchTypeTestCase{
			matchType:       v1alpha2.MatchNotEqual,
			expectedIsRegex: false,
			expectedIsEqual: false,
		}),
		Entry("MatchRegexMatch (=~)", matchTypeTestCase{
			matchType:       v1alpha2.MatchRegexMatch,
			expectedIsRegex: true,
			expectedIsEqual: true,
		}),
		Entry("MatchRegexNotMatch (!~)", matchTypeTestCase{
			matchType:       v1alpha2.MatchRegexNotMatch,
			expectedIsRegex: true,
			expectedIsEqual: false,
		}),
	}

	DescribeTable("should create the correct AM matcher flags",
		func(tc matchTypeTestCase) {
			suffix := sanitizeMatchType(tc.matchType)
			namespace := fmt.Sprintf("test-matchtype-%s", suffix)
			silenceName := fmt.Sprintf("test-mt-%s", suffix)
			expectedComment := "silence-operator-" + namespace + "-" + silenceName

			Expect(createNamespace(ctx, namespace)).To(Succeed())

			silence := &v1alpha2.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      silenceName,
					Namespace: namespace,
				},
				Spec: v1alpha2.SilenceSpec{
					Matchers: []v1alpha2.SilenceMatcher{
						{
							Name:      "alertname",
							Value:     "TestAlert",
							MatchType: tc.matchType,
						},
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

			var amSilence *alertmanager.Silence
			Eventually(func() error {
				var err error
				amSilence, err = findSilenceByComment(amPortForward.localPort, expectedComment)
				if err != nil {
					return err
				}
				if amSilence == nil {
					return fmt.Errorf("silence not found")
				}
				return nil
			}, "60s", "2s").Should(Succeed())

			Expect(amSilence.Matchers).To(HaveLen(1))
			Expect(amSilence.Matchers[0].IsRegex).To(Equal(tc.expectedIsRegex), "isRegex mismatch for %s", tc.matchType)
			Expect(amSilence.Matchers[0].IsEqual).To(Equal(tc.expectedIsEqual), "isEqual mismatch for %s", tc.matchType)
		},
		entries,
	)
})

func sanitizeMatchType(mt v1alpha2.MatchType) string {
	switch mt {
	case v1alpha2.MatchEqual:
		return "eq"
	case v1alpha2.MatchNotEqual:
		return "neq"
	case v1alpha2.MatchRegexMatch:
		return "re"
	case v1alpha2.MatchRegexNotMatch:
		return "nre"
	default:
		return "unknown"
	}
}
