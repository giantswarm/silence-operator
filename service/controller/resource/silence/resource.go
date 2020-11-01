package silence

import (
	"strings"

	"github.com/giantswarm/k8sclient/v4/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
)

const (
	Name = "silence"
)

type Config struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	Targets []string
}

type Resource struct {
	logger micrologger.Logger

	targets map[string]string
}

func New(config Config) (*Resource, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}

	targets := make(map[string]string)
	for _, target := range config.Targets {
		targetObj := strings.Split(target, "=")
		targetName := targetObj[0]
		targetValue := ""
		if len(targetObj) == 2 {
			targetValue = targetObj[1]
		}

		targets[targetName] = targetValue
	}

	r := &Resource{
		logger: config.Logger,

		targets: targets,
	}

	return r, nil
}

func (r *Resource) Name() string {
	return Name
}
