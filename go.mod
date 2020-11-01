module github.com/giantswarm/silence-operator

go 1.14

require (
	github.com/giantswarm/apiextensions/v3 v3.4.2-0.20201101183513-ba493762f75d
	github.com/giantswarm/exporterkit v0.2.0
	github.com/giantswarm/k8sclient/v4 v4.0.0
	github.com/giantswarm/microendpoint v0.2.0
	github.com/giantswarm/microerror v0.2.1
	github.com/giantswarm/microkit v0.2.2
	github.com/giantswarm/micrologger v0.3.3
	github.com/giantswarm/operatorkit/v2 v2.0.2
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/prometheus/client_golang v1.8.0
	github.com/spf13/afero v1.4.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.5.0 // indirect
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	golang.org/x/net v0.0.0-20201002202402-0a1ea396d57c // indirect
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800 // indirect
)

replace (
	github.com/Microsoft/hcsshim v0.8.7 => github.com/Microsoft/hcsshim v0.8.10
	github.com/coreos/etcd v3.3.13+incompatible => github.com/coreos/etcd v3.3.24+incompatible
	sigs.k8s.io/cluster-api v0.3.10 => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/cluster-api-provider-azure v0.4.9 => github.com/giantswarm/cluster-api-provider-azure v0.4.9-gsalpha2
)
