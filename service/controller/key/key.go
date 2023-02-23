package key

import (
	"fmt"
	"time"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

const (
	CreatedBy = "silence-operator"

	ValidUntilLabelName = "valid-until"
)

var (
	// used to create never-ending silence
	eternity = time.Now().AddDate(1000, 0, 0)
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

func SilenceValidUntil(silence v1alpha1.Silence) (time.Time, error) {
	labels := silence.GetLabels()

	value, ok := labels[ValidUntilLabelName]
	if !ok {
		return eternity, nil
	}

	validUntilTime, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, microerror.Maskf(invalidValidUntilDateError, "valid-until date %q does not match expected format %q: %v", value, time.DateOnly, err)
	}

	return validUntilTime, nil
}
