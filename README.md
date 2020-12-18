[![CircleCI](https://circleci.com/gh/giantswarm/silence-operator.svg?&style=shield)](https://circleci.com/gh/giantswarm/silence-operator)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/silence-operator/status "Docker Repository on Quay")](https://quay.io/repository/giantswarm/silence-operator)

# silence-operator

The silence-operator manages [alertmanager](https://github.com/prometheus/alertmanager) alerts.

## Overview

### CustomResourceDefinition

The silence-operator monitors the Kubernetes API server for changes
to `Silence` objects and ensures that the current Alertmanager alerts match these objects.
The Operator reconciles the `Silence` [Custom Resource Definition (CRD)][crd] which
can be found [here][silence-crd].

[crd]: https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/
[silence-crd]: https://github.com/giantswarm/apiextensions/blob/master/pkg/apis/monitoring/v1alpha1/silence_types.go

### How does it work

1. Deployment runs the Kubernetes controller, which reconciles `Silence` CRs.
2. Cronjob runs the synchronization of raw CRs definition from the specified folder by matching tags.

Sample CR:

```yaml
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: test-silence1
spec:
  targetTags:
  - name: installation
    value: kind
  - name: provider
    value: local
  matchers:
  - name: cluster
    value: test
    isRegex: false
```

- `targetTags` field:
  - defines a list of tags, which `sync` command uses to match CRs towards a specific environment
  - each _target tag_ consists of `name` and `value` which is a regexp matched against corresponding `name` tag given on the command line
  - if a `Silence` doesn't specify any `targetTags` it is assumed to match any environment and is synced
  - otherwise for a `Silence` to be synced, all tags defined in its `targetTags` must match all tags given on the `sync` command line

For example, to ensure raw CR, stored at `/folder/cr.yaml`, run:

```bash
silence-operator sync --tag installation=kind --tag provider=local --dir /folder
```

- `matchers` field corresponds to the Alertmanager silence `matchers` each of which consists of:
  - `name` - name of tag on an alert to match
  - `value` - fixed string or expression to match against the value of the tag named by `name` above on an alert
  - `isRegex` - a boolean specifying wheter to treat `value` as a regex or a fixed string

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
