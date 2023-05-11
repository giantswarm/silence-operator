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

var (
	// defaultEndDate is used to create never-ending silence
	defaultEndDate = time.Now().AddDate(100, 0, 0)
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

func SilenceComment(silence v1alpha1.Silence) string {
	return fmt.Sprintf("%s-%s", CreatedBy, silence.Name)
}

// SilenceValidUntil gets the expiry date for a given silence.
// The expiry date is retrieved from the annotation name configured by ValidUntilAnnotationName.
// The expected format is defined by DateOnlyLayout.
// It returns an invalidValidUntilDateError in case the date format is invalid.
func SilenceValidUntil(silence v1alpha1.Silence) (time.Time, error) {
	annotations := silence.GetAnnotations()

	// Check if the annotation exist otherwise return a date 100 years in the future.
	value, ok := annotations[ValidUntilAnnotationName]
	if !ok {
		return defaultEndDate, nil
	}

	// Parse the date found in the annotation.
	validUntilTime, err := time.Parse(DateOnlyLayout, value)
	if err != nil {
		return time.Time{}, microerror.Maskf(invalidValidUntilDateError, "%s date %q does not match expected format %q: %v", ValidUntilAnnotationName, value, DateOnlyLayout, err)
	}

	// We shift the time to 9am CET to ensure silences do not expire at night.
	validUntilTime = time.Date(validUntilTime.Year(), validUntilTime.Month(), validUntilTime.Day(), 9, 0, 0, 0, time.FixedZone("CET", 0))

	return validUntilTime.UTC(), nil
}
