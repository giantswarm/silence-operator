package silence

import (
	"github.com/giantswarm/k8sclient/v8/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

const (
	Name = "silence"
)

type Config struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	AMClient *alertmanager.AlertManager
}

type Resource struct {
	logger micrologger.Logger

	amClient *alertmanager.AlertManager
}

func New(config Config) (*Resource, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}

	r := &Resource{
		logger: config.Logger,

		amClient: config.AMClient,
	}

	return r, nil
}

func (r *Resource) Name() string {
	return Name
}
