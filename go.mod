module github.com/giantswarm/cert-exporter

go 1.14

require (
	github.com/giantswarm/microerror v0.2.0
	github.com/giantswarm/micrologger v0.3.1
	github.com/hashicorp/vault/api v1.0.4
	github.com/prometheus/client_golang v1.6.0
	github.com/spf13/afero v1.2.2
)

replace (
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	golang.org/x/net => golang.org/x/net v0.0.0-20200904194848-62affa334b73
)
