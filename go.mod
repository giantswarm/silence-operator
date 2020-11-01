module github.com/giantswarm/silence-operator

go 1.14

require (
	github.com/giantswarm/apiextensions/v2 v2.6.2
	github.com/giantswarm/apiextensions/v3 v3.4.2-0.20201101183513-ba493762f75d
	github.com/giantswarm/exporterkit v0.2.0
	github.com/giantswarm/k8sclient/v4 v4.0.0
	github.com/giantswarm/k8scloudconfig/v8 v8.1.1
	github.com/giantswarm/kubelock/v2 v2.0.0
	github.com/giantswarm/microendpoint v0.2.0
	github.com/giantswarm/microerror v0.2.1
	github.com/giantswarm/microkit v0.2.2
	github.com/giantswarm/micrologger v0.3.3
	github.com/giantswarm/operatorkit/v2 v2.0.2
	github.com/giantswarm/statusresource/v2 v2.0.0
	github.com/giantswarm/versionbundle v0.2.0
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2
	github.com/markbates/pkger v0.17.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/prometheus/client_golang v1.8.0
	github.com/spf13/afero v1.4.0
	github.com/spf13/viper v1.7.1
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/cluster-api v0.3.10
	sigs.k8s.io/cluster-api-provider-azure v0.4.9
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Microsoft/hcsshim v0.8.7 => github.com/Microsoft/hcsshim v0.8.10
	github.com/coreos/etcd v3.3.13+incompatible => github.com/coreos/etcd v3.3.24+incompatible
	sigs.k8s.io/cluster-api v0.3.10 => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/cluster-api-provider-azure v0.4.9 => github.com/giantswarm/cluster-api-provider-azure v0.4.9-gsalpha2
)
