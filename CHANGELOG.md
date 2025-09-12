# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add enhanced time management fields (`startsAt`, `endsAt`, `duration`) to v1alpha2 Silence CRD with priority-based resolution and backward compatibility.

## [0.18.0] - 2025-07-15

### Added

- Add multi-tenancy infrastructure and configuration support for Alertmanager with configurable tenant extraction and backward compatibility.

## [0.17.0] - 2025-07-02

### Added

- Add advanced filtering capabilities for both v1alpha1 and v1alpha2 controllers:
  - Add silence selector feature to filter `Silence` resources by labels (configure via `--silence-selector` flag).
  - Add namespace selector for v1alpha2 controller to restrict watched namespaces (configure via `--namespace-selector` flag).
- Allow filtering of `Silence` custom resources based on a label selector. The operator will only process `Silence` CRs that match the selector provided via the `--silence-selector` command-line flag or the `silenceSelector` Helm chart value. If no selector is provided, all `Silence` CRs are processed.
- Add new `observability.giantswarm.io/v1alpha2` API with namespace-scoped Silence CRD for improved multi-tenancy.
  - Add `MatchType` enum field using Alertmanager operator symbols (`=`, `!=`, `=~`, `!~`) for intuitive matching logic.
  - Add `SilenceV2Reconciler` controller to handle v1alpha2 resources while maintaining full backward compatibility with v1alpha1.
  - Add comprehensive field validation: matcher names (1-256 chars), values (max 1024 chars), minimum 1 matcher required.
  - Add printer columns to v1alpha2 CRD for better `kubectl get silences` output showing Age.
- Add automated migration script (`hack/migrate-silences.sh`) for v1alpha1 to v1alpha2 conversion.
  - Automatically converts boolean matcher fields (`isRegex`/`isEqual`) to enum format (`matchType`).
  - Intelligently preserves user annotations/labels while filtering out Kubernetes and FluxCD system metadata.
  - Supports dry-run mode for safe migration testing.
- Add comprehensive migration documentation (`MIGRATION.md`) with examples and best practices.
- Add clean service layer architecture (`pkg/service/`) separating business logic from Kubernetes controller concerns.

### Changed

- **BREAKING** (v1alpha2 only): Replace `isRegex` and `isEqual` boolean fields with single `matchType` enum field using Alertmanager symbols.
- **BREAKING** (v1alpha2 only): Change from cluster-scoped to namespace-scoped resources for better multi-tenancy and RBAC isolation.
- **BREAKING** (v1alpha2 only): Remove deprecated fields in v1alpha2: `targetTags`, `owner`, `postmortem_url`, and `issue_url` for cleaner API design.
- Improve code organization with dependency injection and clear separation between controller logic and business logic.

### Deprecated

- The `monitoring.giantswarm.io/v1alpha1` API is now considered legacy. New deployments should use `observability.giantswarm.io/v1alpha2`.

**Migration Note**: Existing v1alpha1 silences continue to work unchanged. Use the automated migration script and see MIGRATION.md for detailed guidance.

## [0.16.1] - 2025-05-20

- Remove duplicate container `securityContext` from the Helm chart deployment template.

## [0.16.0] - 2025-05-20

### Added

- Helm chart now supports conditional installation of the Silence CRD via the `crds.install` value. The CRD is templated and installed by default, but you can disable it by setting `crds.install` to `false`.

## [0.15.0] - 2025-05-14

### Added

- Migrate from Giant Swarm deprecated operatorkit framework to kube-builder.
- Add CiliumNetworkPolicy support.

### Changed

- Migrate from Giant Swarm deprecated `operatorkit` framework to `kube-builder`. This change introduces a few **breaking changes**:
  - Operator configuration has been moved from a configmap to command-line arguments. **This does not affect helm chart users**
  - The operator needs new rbac capabilities to be able manage `leases` and to create `events`
  - http port has been changed from 8000 to 8080.
  - Finalizers set on silences have been changed from `operatorkit.giantswarm.io/silence-operator-silence-controller` to `monitoring.giantswarm.io/silence-protection`
- **helm** `.registry.domain` has been renamed to `image.registry` (**breaking change**)
- **helm** deprecated PodSecurityPolicy has been removed (**breaking change** with kubernetes < 1.25)
- Use `app-build-suite` to build the operator.
- Changed container image from alpine to a non-root distroless image.

### Fixed

- Fixed the linting errors from golangci-lint v2.

### Removed

- Remove the unnecessary sync job to rely on GitOps. **Breaking change**: this means that you should now use your favorite GitOps tool (flux, ArgoCD) to deploy silences on your clusters.
- Removed Giant Swarm legacy `microerrors` package for error handling

## [0.14.1] - 2025-04-23

### Changed

- Bump dependencies and add an example CR for documentation purposes.

## [0.14.0] - 2025-03-31

### Changed

- Make the Silence `isRegex` field optional.

## [0.13.0] - 2025-03-13

- Added option to use Service Account token to be used of Alertmanager authentication.

### Fixed

- Replace Alertmanager RoundTripper with custom NewRequest.
- Fix CVE-2024-45338 by updating golang.org/x/net to v0.33.0

## [0.12.0] - 2024-11-05

### Added

- Add multi-tenancy support via a header for mimir support.
- Allow setting alertmanager url through helm values.

### Changed

- Change CronJob ImagePullPolicy from Always to IfNotPresent to reduce image network traffic.

## [0.11.2] - 2023-12-06

### Changed

- Configure gsoci.azurecr.io as the registry to use by default

## [0.11.1] - 2023-10-12

### Fixed

- Fix real issues and remove policy exception.

## [0.11.0] - 2023-10-02

### Changed

- Add condition for PSP installation in helm chart.
- Add KyvernoPolicyException for sync job.

## [0.10.3] - 2023-08-29

### Fixed

- Support isEqual field on silences.

## [0.10.2] - 2023-08-04

### Fixed

- Fix start and end dates based on the creationTimestamp provided in the SilenceCR.

## [0.10.1] - 2023-07-13

### Added

- Use `securityContext` values inside CronJob template.

### Fixed

- Fix ignored error on accessing the silences.

## [0.10.0] - 2023-06-27

### Added

- Add Kyverno Policy Exceptions.

## [0.9.1] - 2023-05-25

### Added

- Add pod monitor for monitoring purposes.

## [0.9.0] - 2023-05-22

### Added

- Added the use of runtime/default seccomp profile.

### Changed

- updated giantswarm/k8sclient from v6.1.0 to v7.0.1
- updated giantswarm/operatorkit from v6.1.0 to v8.0.0
- Updated sigs.k8s.io/controller-tools from v0.7.0 to v0.11.3
- Updated github.com/spf13/cobra from v1.6.1 to v1.7.0

### Removed

- Stop pushing to `openstack-app-collection`.

## [0.8.0] - 2022-11-08

### Changed

- Set Silence expiry date using value from valid-until label
- Update alpine Docker tag from v3.17.1 to v3.17.2

### Added

- Make Helm chart CronJob optional
- Make Helm chart Alertmanager address configurable
- Make target tags field optional for when sync is disabled
- Only install Helm chart sync secret when sync is enabled
- Only install PodSecurityPolicy on supported Kubernetes versions
- Make Helm chart RBAC deployment optional
- Added the use of runtime/default seccomp profile.

## [0.8.0] - 2022-11-08

### Added

- Add IssueURL field to Silence CRD.

### Changed

- Add `.svc` suffix to the alertmanager address to make silence operator work behind a corporate proxy.
- Upgrade to go 1.19
- Bump github.com/spf13/cobra from 1.4.0 to 1.5.0.
- Bump github.com/spf13/cobra from 1.6.0 to 1.6.1
- Bump sigs.k8s.io/controller-runtime from 0.12.2 to 0.12.3
- Bump sigs.k8s.io/controller-runtime from 0.12.3 to 0.13.0
- Bump sigs.k8s.io/controller-runtime from 0.13.0 to 0.13.1
- Bump alpine from 3.16.0 to 3.17.1
- Bump github.com/prometheus/client_golang from 1.12.2 to 1.13.0
- Bump github.com/prometheus/client_golang from 1.13.1 to 1.14.0
- Bump github.com/giantswarm/k8smetadata from 0.11.1 to 0.13.0
- Reconcile API if Silence CR gets updated
- Deprecate PostmortemURL field in favour of IssueURL.
- Make Silence Owner field a string instead of string pointer.
- Bump golang.org/x/text from v0.3.7 to v0.3.8
- Bump github.com/nats-io/nats-server from v2.5.0 to v2.9.3
- Bump github.com/getsentry/sentry-go from v0.11.0 to v0.14.0

## [0.7.0] - 2022-06-13

### Added

- Support update of silences.

### Changed

- Dependencies updates, solves some of Nancy security alerts
- Set `startingDeadlineSeconds` to 240 seconds to ensure it is scheduled and to avoid `FailedNeedsStart` events.

## [0.6.1] - 2022-04-12

### Added

- Push `silence-operator` to gcp-app-collection.

### Fixed

- Make optional fields really optional.

## [0.6.0] - 2022-04-12

### Added

- Add silence owner (GitHub username).
- Add postmortem URL.

## [0.5.0] - 2022-03-28

### Changed

- Wire to new alertmanager.

## [0.4.0] - 2021-11-29

### Changed

- Update `operatorkit` to v6.
- Update `k8sclient` to v6.
- Move Silence API from `apiextensions` to this repository.

## [0.3.0] - 2021-11-11

### Added

- Respect `giantswarm.io/keep: true` annotations on Silences when performing the initial cleanup

## [0.2.2] - 2021-08-13

### Changed

- Add support for the negative silence matchers.

## [0.2.1] - 2021-06-21

### Fixed

- Use `--depth=1` when cloning silences repository.

## [0.2.0] - 2021-05-25

### Changed

- Prepare helm values for configuration management.
- Update architect-orb to v3.0.0.

## [0.1.5] - 2021-03-30

### Changed

- Use `restartPolicy: OnFailure` for syncing silences cronjob.

## [0.1.4] - 2020-11-18

### Fixed

- Fix `create` event in silence controller.

## [0.1.3] - 2020-11-17

### Changed

- Run synchronization job every 5min.

## [0.1.2] - 2020-11-09

### Fixed

- Handle gracefully `Silence` CR deletion if Alertmanager alert doesn't exist.

## [0.1.1] - 2020-11-09

### Changed

- Deploy app into `monitoring` namespace.

## [0.1.0] - 2020-11-09

- Add `silence` controller.
- Add `sync` command.
- Push `silence-operator` to app-collections.

[Unreleased]: https://github.com/giantswarm/silence-operator/compare/v0.18.0...HEAD
[0.18.0]: https://github.com/giantswarm/silence-operator/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/giantswarm/silence-operator/compare/v0.16.1...v0.17.0
[0.16.1]: https://github.com/giantswarm/silence-operator/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/giantswarm/silence-operator/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/giantswarm/silence-operator/compare/v0.14.1...v0.15.0
[0.14.1]: https://github.com/giantswarm/silence-operator/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/giantswarm/silence-operator/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/giantswarm/silence-operator/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/giantswarm/silence-operator/compare/v0.11.2...v0.12.0
[0.11.2]: https://github.com/giantswarm/silence-operator/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/giantswarm/silence-operator/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/giantswarm/silence-operator/compare/v0.10.3...v0.11.0
[0.10.3]: https://github.com/giantswarm/silence-operator/compare/v0.10.2...v0.10.3
[0.10.2]: https://github.com/giantswarm/silence-operator/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/giantswarm/silence-operator/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/giantswarm/silence-operator/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/giantswarm/silence-operator/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/giantswarm/silence-operator/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/giantswarm/silence-operator/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/giantswarm/silence-operator/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/giantswarm/silence-operator/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/giantswarm/silence-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/giantswarm/silence-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/silence-operator/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/silence-operator/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/giantswarm/silence-operator/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/silence-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/silence-operator/compare/v0.1.5...v0.2.0
[0.1.5]: https://github.com/giantswarm/silence-operator/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/giantswarm/silence-operator/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/giantswarm/silence-operator/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/giantswarm/silence-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/silence-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/silence-operator/releases/tag/v0.1.0
