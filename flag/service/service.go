package service

import (
	"github.com/giantswarm/operatorkit/v8/pkg/flag/service/kubernetes"

	"github.com/giantswarm/silence-operator/flag/service/alertmanager"
)

// Service is an intermediate data structure for command line configuration flags.
type Service struct {
	AlertManager alertmanager.AlertManager
	Kubernetes   kubernetes.Kubernetes
}
