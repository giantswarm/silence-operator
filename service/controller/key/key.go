package key

import (
	monitoringv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/monitoring/v1alpha1"
	"github.com/giantswarm/microerror"
)

const (
	CreatedBy = "silence-operator"
)

func ToSilence(v interface{}) (monitoringv1alpha1.Silence, error) {
	if v == nil {
		return monitoringv1alpha1.Silence{}, microerror.Maskf(wrongTypeError, "expected non-nil, got %#v", v)
	}

	p, ok := v.(*monitoringv1alpha1.Silence)
	if !ok {
		return monitoringv1alpha1.Silence{}, microerror.Maskf(wrongTypeError, "expected %T, got %T", p, v)
	}

	c := p.DeepCopy()

	return *c, nil
}
