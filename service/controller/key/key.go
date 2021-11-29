package key

import (
	"fmt"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

const (
	CreatedBy = "silence-operator"
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
