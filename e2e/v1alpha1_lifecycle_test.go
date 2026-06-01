//go:build e2e

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

var _ = Describe("v1alpha1 Silence lifecycle", func() {
	ctx := context.Background()

	It("should create a silence in Alertmanager when a v1alpha1 Silence CR is created", func() {
		const silenceName = "test-v1-create"
		expectedComment := "silence-operator-" + silenceName

		silence := &v1alpha1.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name: silenceName,
			},
			Spec: v1alpha1.SilenceSpec{
				Matchers: []v1alpha1.Matcher{
					{
						Name:  "instance",
						Value: "test-v1alpha1",
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
		Expect(amSilence.Matchers[0].Value).To(Equal("test-v1alpha1"))
	})

	It("should remove the silence from Alertmanager when the v1alpha1 Silence CR is deleted", func() {
		const silenceName = "test-v1-delete"
		expectedComment := "silence-operator-" + silenceName

		silence := &v1alpha1.Silence{
			ObjectMeta: metav1.ObjectMeta{
				Name: silenceName,
			},
			Spec: v1alpha1.SilenceSpec{
				Matchers: []v1alpha1.Matcher{
					{
						Name:  "instance",
						Value: "test-v1alpha1-delete",
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
