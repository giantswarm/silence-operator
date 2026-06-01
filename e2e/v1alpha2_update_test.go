//go:build e2e

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

var _ = Describe("v1alpha2 Silence update", func() {
	const (
		namespace   = "test-v2-update"
		silenceName = "test-update"
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

	It("should update the Alertmanager silence when the Silence CR matchers are updated", func() {
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{
						Name:      "env",
						Value:     "staging",
						MatchType: v1alpha2.MatchEqual,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())

		// Wait for the silence to appear in AM with original value.
		Eventually(func() string {
			s, _ := findSilenceByComment(amPortForward.localPort, expectedComment)
			if s == nil || len(s.Matchers) == 0 {
				return ""
			}
			return s.Matchers[0].Value
		}, "60s", "2s").Should(Equal("staging"))

		// Update the matcher value.
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence)).To(Succeed())
		silence.Spec.Matchers[0].Value = "production"
		Expect(k8sClient.Update(ctx, silence)).To(Succeed())

		// Verify AM silence is updated.
		Eventually(func() string {
			s, _ := findSilenceByComment(amPortForward.localPort, expectedComment)
			if s == nil || len(s.Matchers) == 0 {
				return ""
			}
			return s.Matchers[0].Value
		}, "60s", "2s").Should(Equal("production"))
	})
})
