package controller

import (
	"github.com/giantswarm/k8sclient/v7/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/v7/pkg/controller"
	"github.com/giantswarm/operatorkit/v7/pkg/resource"
	"github.com/giantswarm/operatorkit/v7/pkg/resource/wrapper/metricsresource"
	"github.com/giantswarm/operatorkit/v7/pkg/resource/wrapper/retryresource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha1"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/project"
	"github.com/giantswarm/silence-operator/service/controller/resource/silence"
)

type SilenceConfig struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	AlertManagerAddress        string
	AlertManagerAuthentication bool
	AlertManagerTenant         string
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
			NewRuntimeObjectFunc: func() client.Object {
				return new(v1alpha1.Silence)
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

	var amClient *alertmanager.AlertManager
	{
		amConfig := alertmanager.Config{
			Address:        config.AlertManagerAddress,
			Authentication: config.AlertManagerAuthentication,
			BearerToken:    config.K8sClient.RESTConfig().BearerToken,
			TenantId:       config.AlertManagerTenant,
		}

		amClient, err = alertmanager.New(amConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var silenceResource resource.Interface
	{
		c := silence.Config{
			K8sClient: config.K8sClient,
			Logger:    config.Logger,

			AMClient: amClient,
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
