package key

import (
	"fmt"
	"time"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

const (
	CreatedBy                = "silence-operator"
	ValidUntilAnnotationName = "valid-until"
	DateOnlyLayout           = "2006-01-02"
)

func ToSilence(v interface{}) (v1alpha1.Silence, error) {
	if v == nil {
		return v1alpha1.Silence{}, microerror.Maskf(wrongTypeError, "expected non-nil, got %#v", v)
	}

	p, ok := v.(*v1alpha1.Silence)
	if !ok {
		return v1alpha1.Silence{}, microerror.Maskf(wrongTypeError, "expected %T, got %T", p, v)
	}

	c := p.DeepCopy()

	return *c, nil
}

func SilenceComment(silence *v1alpha1.Silence) string {
	return fmt.Sprintf("%s-%s", CreatedBy, silence.Name)
}

// SilenceEndsAt gets the expiry date for a given silence.
// The expiry date is retrieved from the annotation name configured by ValidUntilAnnotationName.
// The expected format is defined by DateOnlyLayout.
// It returns an invalidExpirationDateError in case the date format is invalid.
func SilenceEndsAt(silence *v1alpha1.Silence) (time.Time, error) {
	annotations := silence.GetAnnotations()

	// Check if the annotation exist otherwise return a date 100 years in the future.
	value, ok := annotations[ValidUntilAnnotationName]
	if !ok {
		return silence.GetCreationTimestamp().AddDate(100, 0, 0), nil
	}

	// Parse the date found in the annotation using RFC3339 (ISO8601) by default
	expirationDate, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// If it fails, we try to parse it using date only (old way)
		expirationDate, err = time.Parse(DateOnlyLayout, value)
		if err != nil {
			return time.Time{}, microerror.Maskf(invalidExpirationDateError, "%s date %q does not match expected format %q: %v", ValidUntilAnnotationName, value, DateOnlyLayout, err)
		}
		// We shift the time to 8am UTC (9 CET or 10 CEST) to ensure silences do not expire at night.
		expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), 8, 0, 0, 0, time.UTC)
	}

	return expirationDate, nil
}
