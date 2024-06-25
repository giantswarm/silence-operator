# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Added option to use Service Account token for Alertmanager authentication.

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
- Make Helm chart AlertManager address configurable
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

[Unreleased]: https://github.com/giantswarm/silence-operator/compare/v0.11.2...HEAD
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
