# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project's packages adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.7.0] - 2021-05-26

### Changed

- Prepare helm values to configuration management.
- Update architect-orb to v3.0.0.

## [1.6.1] - 2021-03-26

### Changed

- Set docker.io as the default registry

## [1.6.0] - 2021-01-27

### Added

- Add exceptions in NetworkPolicies to allow DNS to work correctly through port 53.

## [1.5.0] - 2021-01-05

### Changed

- Check ca.crt expiries in TLS secrets. ([#109](https://github.com/giantswarm/cert-exporter/pull/109))

## [1.4.0] - 2020-12-02

### Added

- Add new metric (`cert_exporter_secret_not_after`) which tracks expiry of TLS certificates stored in Kubernetes secrets. ([#92](https://github.com/giantswarm/cert-exporter/pull/92))

## [1.3.0] - 2020-09-17

### Added

- Add Network Policy.

### Changed

- Remove `hostNetwork` and `hostPID` capabilities.

## [1.2.4] - 2020-08-13

### Fixed

- Adjusted vault token format check for base62 tokens.

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

## v1.1.0 2019-07-17

### Changed

- Tolerations changed to tolerate all taints.
- Change priority class to `giantswarm-critical`.

[Unreleased]: https://github.com/giantswarm/cert-exporter/compare/v1.7.0...HEAD
[1.7.0]: https://github.com/giantswarm/cert-exporter/compare/v1.6.1...v1.7.0
[1.6.1]: https://github.com/giantswarm/cert-exporter/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/giantswarm/cert-exporter/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/giantswarm/cert-exporter/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/giantswarm/cert-exporter/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/giantswarm/cert-exporter/compare/v1.2.4...v1.3.0
[1.2.4]: https://github.com/giantswarm/cert-exporter/compare/v1.2.3...v1.2.4
[v1.2.3]: https://github.com/giantswarm/cert-exporter/compare/v1.2.2...v1.2.3
[v1.2.2]: https://github.com/giantswarm/cert-exporter/compare/v1.2.1...v1.2.2
[v1.2.1]: https://github.com/giantswarm/cert-exporter/compare/v1.2.0...v1.2.1
[v1.2.0]: https://github.com/giantswarm/cert-exporter/releases/tag/v1.2.0
