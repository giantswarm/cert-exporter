module github.com/giantswarm/cert-exporter

go 1.14

require (
	github.com/giantswarm/k8sclient/v5 v5.0.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.5.0
	github.com/hashicorp/vault/api v1.0.4
	github.com/prometheus/client_golang v1.8.0
	github.com/spf13/afero v1.5.1
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
)

replace github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2

replace github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
