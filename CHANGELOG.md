# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project's packages adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.8.3] - 2023-10-26

### Fixed

- Fix daemonset and deployment Kyverno PolicyException.

## [2.8.2] - 2023-10-23

### Changed

- Fix daemonset Kyverno PolicyException namespace.

## [2.8.1] - 2023-10-23

### Changed

- Make Kyverno PolicyExceptions configurable.

## [2.8.0] - 2023-10-18

### Changed

- Replace condition for PSP CR installation.

## [2.7.0] - 2023-09-27

### Changed

- Add Service Monitor.

## [2.6.0] - 2023-06-01

### Changed

- Remove the `Exist` toleration from deployment. This allows the pod to be rescheduled on a drained node sometimes causing the drain of a node to fail and require a manual fix

## [2.5.1] - 2023-05-04

### Changed

- Allow requests from the api-server.

## [2.5.0] - 2023-05-04

### Changed

- Update icon
- Disable PSPs for k8s 1.25 and newer.

## [2.4.0] - 2023-04-03

### Added

- Add cilium network policies.

## [2.3.1] - 2022-12-13

### Fixed

- Allow eviction for cert-exporter-deployment.

## [2.3.0] - 2022-09-05

### Changed

- Update base container image to quay.io/giantswarm/alpine:3.16.2-giantswarm.
- Update go to 1.18.
- Update github.com/giantswarm/k8sclient to v7.0.1.
- Update github.com/hashicorp/vault/api to v1.7.2.
- Update github.com/prometheus/client_golang to v1.13.0.
- Update github.com/spf13/afero to v1.9.2.
- Update k8s.io/api to v0.23.10.
- Update k8s.io/apimachinery to v0.23.10.
- Update k8s.io/client-go to v0.23.10.

### Added

- Add /etc/kubernetes/pki to --cert-paths flag in DaemonSet deployment.

## [2.2.0] - 2022-03-24

### Changed

- Change priorityClass to `system-node-critical` for the daemonset.

## [2.1.1] - 2022-03-16

### Fixed

- Allow egress to port 1053 to make in-cluster DNS queries work.
- Allow egress to port 443 to allow accessing vault.

## [2.1.0] - 2022-02-01

### Changed

- Make exporter's monitor flags configurable.

## [2.0.1] - 2021-12-15

### Changed

- Equalise labels in the helm chart.

## [2.0.0] - 2021-10-20

### Changed

- Export presence of `giantswarm.io/service-type: managed` label in cert-manager `Issuer` and `ClusterIssuer` CR referenced by `Certificate` CR `issuerRef` spec field to `cert_exporter_certificate_cr_not_after` metric as `managed_issuer` label.
- Add `--monitor-files` and `--monitor-secrets` flags.
- Add Deployment to helm chart to avoid exporting secrets and certificate metrics from DaemonSets.
- Build container image using retagged giantswarm alpine.
- Run as non-root inside container.

## [1.8.0] - 2021-08-25

### Added

- Add new `cert_exporter_certificate_cr_not_after` metric. This metric exports the `status.notAfter` field of cert-manager `Certificate` CR.

### Changed

- Remove static certificate source label from `cert_exporter_secret_not_after` (static value `secret`) and `cert_exporter_not_after` (static value `file`) metrics.

## [1.7.1] - 2021-05-26

### Fixed

- Fix configuration version in `Chart.yaml`.

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

[Unreleased]: https://github.com/giantswarm/cert-exporter/compare/v2.8.3...HEAD
[2.8.3]: https://github.com/giantswarm/cert-exporter/compare/v2.8.2...v2.8.3
[2.8.2]: https://github.com/giantswarm/cert-exporter/compare/v2.8.1...v2.8.2
[2.8.1]: https://github.com/giantswarm/cert-exporter/compare/v2.8.0...v2.8.1
[2.8.0]: https://github.com/giantswarm/cert-exporter/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/giantswarm/cert-exporter/compare/v2.6.0...v2.7.0
[2.6.0]: https://github.com/giantswarm/cert-exporter/compare/v2.6.0...v2.6.0
[2.6.0]: https://github.com/giantswarm/cert-exporter/compare/v2.5.1...v2.6.0
[2.5.1]: https://github.com/giantswarm/cert-exporter/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/giantswarm/cert-exporter/compare/v2.4.0...v2.5.0
[2.4.0]: https://github.com/giantswarm/cert-exporter/compare/v2.3.1...v2.4.0
[2.3.1]: https://github.com/giantswarm/cert-exporter/compare/v2.3.0...v2.3.1
[2.3.0]: https://github.com/giantswarm/cert-exporter/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/giantswarm/cert-exporter/compare/v2.1.1...v2.2.0
[2.1.1]: https://github.com/giantswarm/cert-exporter/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/giantswarm/cert-exporter/compare/v2.0.1...v2.1.0
[2.0.1]: https://github.com/giantswarm/cert-exporter/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/giantswarm/cert-exporter/compare/v1.8.0...v2.0.0
[1.8.0]: https://github.com/giantswarm/cert-exporter/compare/v1.7.1...v1.8.0
[1.7.1]: https://github.com/giantswarm/cert-exporter/compare/v1.7.0...v1.7.1
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
