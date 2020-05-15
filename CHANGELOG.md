# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project's packages adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.2.3] 2020-05-15

### Changed
- Update prometheus/client_golang dependency
- Migrate from dep to go modules
- Move to App deployment

## [v1.2.2] 2020-04-01

### Changed

- Change daemonset to use release revision not time for Helm 3 support.

## [v1.2.1] 2019-12-24

### Changed

- Remove CPU limits.

## [v1.2.0] 2019-10-23

### Added

- Push cert-exporter to default app catalog.

## [v1.1.0] 2019-07-17

### Changed

- Tolerations changed to tolerate all taints.
- Change priority class to `giantswarm-critical`.

[Unreleased]: https://github.com/giantswarm/cert-exporter/compare/v1.2.2...HEAD
[v1.2.2]: https://github.com/giantswarm/cert-exporter/releases/tag/v1.2.2
[v1.2.1]: https://github.com/giantswarm/cert-exporter/releases/tag/v1.2.1
[v1.2.0]: https://github.com/giantswarm/cert-exporter/releases/tag/v1.2.0
[v1.2.0]: https://github.com/giantswarm/cert-exporter/releases/tag/v1.1.0
