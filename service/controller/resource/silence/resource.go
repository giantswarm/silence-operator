package silence

import (
	"strings"
	"time"

	"github.com/giantswarm/k8sclient/v4/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

const (
	Name = "silence"

	createdBy = "silence-operator"
)

var (
	// used to create never-ending silence
	eternity = time.Now().AddDate(1000, 0, 0)
)

type Config struct {
	K8sClient k8sclient.Interface
	Logger    micrologger.Logger

	AMClient *alertmanager.AlertManager
	Tags     []string
}

type Resource struct {
	logger micrologger.Logger

	amClient *alertmanager.AlertManager
	tags     map[string]string
}

func New(config Config) (*Resource, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}

	tags := make(map[string]string)
	for _, tag := range config.Tags {
		tagObj := strings.Split(tag, "=")
		tagName := tagObj[0]
		tagValue := ""
		if len(tagObj) == 2 {
			tagValue = tagObj[1]
		}

		tags[tagName] = tagValue
	}

	r := &Resource{
		logger: config.Logger,

		amClient: config.AMClient,
		tags:     tags,
	}

	return r, nil
}

func (r *Resource) Name() string {
	return Name
}
