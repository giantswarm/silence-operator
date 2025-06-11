package config

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
)

func TestParseSilenceSelector(t *testing.T) {
	g := gomega.NewWithT(t)

	t.Run("empty selector returns nil", func(t *testing.T) {
		selector, err := ParseSilenceSelector("")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(selector).To(gomega.BeNil())
	})

	t.Run("valid single label selector", func(t *testing.T) {
		selector, err := ParseSilenceSelector("environment=production")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(selector).ToNot(gomega.BeNil())
		g.Expect(selector.String()).To(gomega.Equal("environment=production"))
	})

	t.Run("valid multiple label selector", func(t *testing.T) {
		selector, err := ParseSilenceSelector("environment=production,tier=frontend")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(selector).ToNot(gomega.BeNil())
		g.Expect(selector.String()).To(gomega.Equal("environment=production,tier=frontend"))
	})

	t.Run("valid complex selector with not in", func(t *testing.T) {
		selector, err := ParseSilenceSelector("environment notin (staging)")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(selector).ToNot(gomega.BeNil())
		g.Expect(selector.String()).To(gomega.Equal("environment notin (staging)"))
	})

	t.Run("invalid selector returns error", func(t *testing.T) {
		selector, err := ParseSilenceSelector("invalid=label=selector")
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(selector).To(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.ContainSubstring("unable to parse silence-selector string"))
	})

	t.Run("selector with set-based requirements", func(t *testing.T) {
		selector, err := ParseSilenceSelector("environment in (production,staging)")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(selector).ToNot(gomega.BeNil())
		g.Expect(selector.String()).To(gomega.Equal("environment in (production,staging)"))
	})

	t.Run("selector matches expected labels", func(t *testing.T) {
		selector, err := ParseSilenceSelector("environment=production,tier=frontend")
		g.Expect(err).ToNot(gomega.HaveOccurred())

		// Test matching labels
		matchingLabels := labels.Set{
			"environment": "production",
			"tier":        "frontend",
			"app":         "some-app",
		}
		g.Expect(selector.Matches(matchingLabels)).To(gomega.BeTrue())

		// Test non-matching labels
		nonMatchingLabels := labels.Set{
			"environment": "staging", // Different value
			"tier":        "frontend",
		}
		g.Expect(selector.Matches(nonMatchingLabels)).To(gomega.BeFalse())
	})
}
