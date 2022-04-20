package sync

import (
	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagDir = "dir"
	flagTag = "tag"

	flagKubernetesInCluster  = "kubernetes.incluster"
	flagKubernetesKubeconfig = "kubernetes.kubeconfig"
)

type flag struct {
	Dirs []string
	Tags []string

	KubernetesInCluster  bool
	KubernetesKubeconfig string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&f.Dirs, flagDir, []string{}, "Directory to look for yaml with silence CRs.")
	cmd.Flags().StringSliceVar(&f.Tags, flagTag, []string{}, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")

	cmd.Flags().BoolVar(&f.KubernetesInCluster, flagKubernetesInCluster, false, "Whether to use the in-cluster config to authenticate with Kubernetes.")
	cmd.Flags().StringVar(&f.KubernetesKubeconfig, flagKubernetesKubeconfig, "", "KubeConfig used to connect to Kubernetes.")
}

func (f *flag) Validate() error {
	if len(f.Dirs) == 0 {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty", flagDir)
	}

	if !f.KubernetesInCluster && f.KubernetesKubeconfig == "" {
		return microerror.Maskf(invalidFlagError, "--%s must not be empty in kubernetes.incluster=false", flagKubernetesKubeconfig)
	}

	return nil
}
