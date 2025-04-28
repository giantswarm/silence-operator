[![CircleCI](https://circleci.com/gh/giantswarm/silence-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/silence-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/silence-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/silence-operator)

# silence-operator

The silence-operator manages [alertmanager](https://github.com/prometheus/alertmanager) [silences](https://prometheus.io/docs/alerting/latest/alertmanager/#silences).

## Overview

### CustomResourceDefinition

The silence-operator monitors the Kubernetes API server for changes
to `Silence` objects and ensures that the current Alertmanager silences match these objects.
The Operator reconciles the `Silence` [Custom Resource Definition (CRD)][crd] which
can be found [here][silence-crd].

The `Silence` CRD generated at [config/crd/monitoring.giantswarm.io_silences.yaml](config/crd/monitoring.giantswarm.io_silences.yaml) is deployed via [management-cluster-bases](https://github.com/giantswarm/management-cluster-bases/blob/9e17d416dd324e07d7784054237302707ba42dc3/bases/crds/giantswarm/kustomization.yaml#L6C1-L7C1) repository.

[crd]: https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/
[silence-crd]: api/v1alpha1/silence_types.go

### How does it work

Deployment runs the Kubernetes controller, which reconciles `Silence` CRs.

Sample CR:

```yaml
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: test-silence1
spec:
  matchers:
  - name: cluster
    value: test
    isRegex: false
```

- `matchers` field corresponds to the Alertmanager silence `matchers` each of which consists of:
  - `name` - name of tag on an alert to match
  - `value` - fixed string or expression to match against the value of the tag named by `name` above on an alert
  - `isRegex` - a boolean specifying whether to treat `value` as a regex (`=~`) or a fixed string (`=`)
  - `isEqual` - a boolean specifying whether to use equal signs (`=` or `=~`) or to negate the matcher (`!=` or `!~`)

## Getting the Project

Download the latest release:
https://github.com/giantswarm/silence-operator/releases/latest

Clone the git repository: https://github.com/giantswarm/silence-operator.git

Download the latest docker image from here:
https://quay.io/repository/giantswarm/silence-operator

### How to build

Build the standard way.

```
go build github.com/giantswarm/silence-operator
```

## Contact

- Mailing list: [giantswarm](https://groups.google.com/forum/!forum/giantswarm)
- Bugs: [issues](https://github.com/giantswarm/silence-operator/issues)

## Contributing & Reporting Bugs

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches, the
contribution workflow as well as reporting bugs.

For security issues, please see [the security policy](SECURITY.md).


## License

silence-operator is under the Apache 2.0 license. See the [LICENSE](LICENSE) file
for details.


## Credit
- https://golang.org
- https://github.com/giantswarm/microkit
