[![CircleCI](https://circleci.com/gh/giantswarm/cert-exporter.svg?&style=shield)](https://circleci.com/gh/giantswarm/cert-exporter)

# cert-exporter

Exposes two metrics to Prometheus reagrding certificates/tokens:

## `cert_exporter_not_after`

Timestamp after which the cert is invalid

## `cert_operator_vault_token_expire_time`

Timestamp after which the Vault token is expired

## Deployment

* Managed by [app-operator].
* Production releases are stored in the [default-catalog].
* WIP releases are stored in the [default-test-catalog].

## Installing the Chart

To install the chart locally:

```bash
$ git clone https://github.com/giantswarm/cert-exporter.git
$ cd cert-exporter
$ helm install helm/cert-exporter
```

Provide a custom `values.yaml`:

```bash
$ helm install cert-exporter -f values.yaml
```

## Release Process

* Ensure CHANGELOG.md is up to date.
* Create a new GitHub release with the version e.g. `v0.1.0` and link the
changelog entry.
* This will push a new git tag and trigger a new tarball to be pushed to the
[default-catalog].
* Update [cluster-operator] with the new version.

[app-operator]: https://github.com/giantswarm/app-operator
[cluster-operator]: https://github.com/giantswarm/cluster-operator
[default-catalog]: https://github.com/giantswarm/default-catalog
[default-test-catalog]: https://github.com/giantswarm/default-test-catalog
