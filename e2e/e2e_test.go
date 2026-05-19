//go:build e2e

package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	k8sClient, err = newK8sClient()
	Expect(err).NotTo(HaveOccurred())

	amPortForward, err = startPortForward("alertmanager", "svc/alertmanager", 9093)
	Expect(err).NotTo(HaveOccurred())

	// Wait for port-forward to be ready.
	Eventually(func() error {
		_, err := getActiveSilences(amPortForward.localPort)
		return err
	}, "30s", "1s").Should(Succeed())
})

var _ = AfterSuite(func() {
	if amPortForward != nil {
		amPortForward.stop()
	}
})
