//go:build e2e

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

var _ = Describe("Silence expiry", func() {
	const (
		namespace   = "test-expiry"
		silenceName = "test-expired"
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

	It("should not create an AM silence when valid-until is in the past", func() {
		silence := &v1alpha2.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name:      silenceName,
				Namespace: namespace,
				Annotations: map[string]string{
					alertmanager.ValidUntilAnnotationName: "2020-01-01",
				},
			},
			Spec: v1alpha2.SilenceSpec{
				Matchers: []v1alpha2.SilenceMatcher{
					{
						Name:      "alertname",
						Value:     "ShouldNotAppear",
						MatchType: v1alpha2.MatchEqual,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())

		Consistently(func() *alertmanager.Silence {
			s, _ := findSilenceByComment(amPortForward.localPort, expectedComment)
			return s
		}, "15s", "2s").Should(BeNil())
	})
})
