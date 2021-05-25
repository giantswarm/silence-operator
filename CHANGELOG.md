# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/silence-operator/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/giantswarm/silence-operator/compare/v0.1.5...v0.2.0
[0.1.5]: https://github.com/giantswarm/silence-operator/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/giantswarm/silence-operator/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/giantswarm/silence-operator/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/giantswarm/silence-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/silence-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/silence-operator/releases/tag/v0.1.0
