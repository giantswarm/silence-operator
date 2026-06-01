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

var _ = Describe("v1alpha2 Silence lifecycle", func() {
	ctx := context.Background()

	It("should create a silence in Alertmanager when a Silence CR is created", func() {
		const (
			namespace   = "test-v2-create"
			silenceName = "test-create"
		)
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
						Name:      "instance",
						Value:     "test-v1alpha2",
						MatchType: v1alpha2.MatchEqual,
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
				return fmt.Errorf("silence not found in Alertmanager")
			}
			return nil
		}, "60s", "2s").Should(Succeed())

		Expect(amSilence.Matchers).To(HaveLen(1))
		Expect(amSilence.Matchers[0].Name).To(Equal("instance"))
		Expect(amSilence.Matchers[0].Value).To(Equal("test-v1alpha2"))
	})

	It("should remove the silence from Alertmanager when the Silence CR is deleted", func() {
		const (
			namespace   = "test-v2-delete"
			silenceName = "test-delete"
		)
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
						Name:      "instance",
						Value:     "test-delete",
						MatchType: v1alpha2.MatchEqual,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, silence)).To(Succeed())

		Eventually(func() *alertmanager.Silence {
			s, _ := findSilenceByComment(amPortForward.localPort, expectedComment)
			return s
		}, "60s", "2s").ShouldNot(BeNil())

		Expect(k8sClient.Delete(ctx, silence)).To(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(silence), silence)
			return err != nil
		}, "60s", "2s").Should(BeTrue())

		Eventually(func() *alertmanager.Silence {
			s, _ := findSilenceByComment(amPortForward.localPort, expectedComment)
			return s
		}, "60s", "2s").Should(BeNil())
	})
})
