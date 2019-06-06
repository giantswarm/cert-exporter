[![CircleCI](https://circleci.com/gh/giantswarm/cert-exporter.svg?&style=shield&circle-token=0b5cc07c7114258992b3411a963ce9515e32106a)](https://circleci.com/gh/giantswarm/cert-exporter)

# cert-exporter

Exposes two metrics to Prometheus reagrding certificates/tokens:

## `cert_exporter_not_after`

Timestamp after which the cert is invalid

## `cert_operator_vault_token_expire_time`

Timestamp after which the Vault token is expired
