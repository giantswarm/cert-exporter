# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project's packages adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Change daemonset to use release revision not time for Helm 3 support.

## [1.2.1] 2019-12-24

### Changed

- Remove CPU limits.

## [1.2.0] 2019-10-23

### Added

- Push cert-exporter to default app catalog.

## [1.1.0] 2019-07-17

### Changed

- Tolerations changed to tolerate all taints.
- Change priority class to `giantswarm-critical`.

[Unreleased]: https://github.com/giantswarm/cert-exporter/compare/v1.1.0...HEAD
