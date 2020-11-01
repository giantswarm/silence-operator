package controller

import (
	monitoringv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/monitoring/v1alpha1"
	"github.com/giantswarm/k8sclient/v4/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/v2/pkg/controller"
	"github.com/giantswarm/operatorkit/v2/pkg/resource"
	"github.com/giantswarm/operatorkit/v2/pkg/resource/wrapper/metricsresource"
	"github.com/giantswarm/operatorkit/v2/pkg/resource/wrapper/retryresource"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/giantswarm/silence-operator/pkg/project"
	"github.com/giantswarm/silence-operator/service/controller/resource/silence"
)

type SilenceConfig struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	Targets []string
}

type Silence struct {
	*controller.Controller
}

func NewSilence(config SilenceConfig) (*Silence, error) {
	var err error

	resources, err := newSilenceResources(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var operatorkitController *controller.Controller
	{
		c := controller.Config{
			K8sClient: config.K8sClient,
			Logger:    config.Logger,
			NewRuntimeObjectFunc: func() runtime.Object {
				return new(monitoringv1alpha1.Silence)
			},
			Resources: resources,

			Name: project.Name() + "-silence-controller",
		}

		operatorkitController, err = controller.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	c := &Silence{
		Controller: operatorkitController,
	}

	return c, nil
}

func newSilenceResources(config SilenceConfig) ([]resource.Interface, error) {
	var err error

	var silenceResource resource.Interface
	{
		c := silence.Config{
			K8sClient: config.K8sClient,
			Logger:    config.Logger,
			Targets:   config.Targets,
		}

		silenceResource, err = silence.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	resources := []resource.Interface{
		silenceResource,
	}

	{
		c := retryresource.WrapConfig{
			Logger: config.Logger,
		}

		resources, err = retryresource.Wrap(resources, c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	{
		c := metricsresource.WrapConfig{}

		resources, err = metricsresource.Wrap(resources, c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return resources, nil
}
