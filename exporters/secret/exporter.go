package secret

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/giantswarm/k8sclient/v5/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	fieldSelector = "type=kubernetes.io/tls"
)

type Config struct {
	Namespaces []string
}

type Exporter struct {
	cert      *prometheus.Desc
	ctx       context.Context
	k8sClient *kubernetes.Clientset
	logger    micrologger.Logger

	namespaces []string
}

func DefaultConfig() Config {
	return Config{
		Namespaces: []string{},
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "start collecting metrics")

	// If no namespace whitelist is provided then we check all namespaces
	if len(e.namespaces) == 0 {
		allNamespaces, err := e.k8sClient.CoreV1().Namespaces().List(e.ctx, metav1.ListOptions{})
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}

		for _, ns := range allNamespaces.Items {
			// We just need the namespace's name
			nsName := ns.Name
			e.namespaces = append(e.namespaces, nsName)
		}
	}

	listOpts := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}

	// Range over namespaces
	for _, namespace := range e.namespaces {
		// Get secrets in namespace
		secrets, err := e.k8sClient.CoreV1().Secrets(namespace).List(e.ctx, listOpts)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}
		// Range over secrets
		for _, secret := range secrets.Items {
			err := e.calculateExpiry(ch, namespace, secret)
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
			}
		}
	}

	e.logger.Log("info", "finished collecting metrics")
}

func (e *Exporter) calculateExpiry(ch chan<- prometheus.Metric, namespace string, secret v1.Secret) error {
	secretName := secret.Name
	certBytes := secret.Data["tls.crt"]

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return nil
	}

	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		e.logger.Log("warning", fmt.Sprintf("%s could not be parsed as a certificate: %s", secretName, microerror.Mask(err)))
		return nil
	}

	for _, cert := range certs {
		timestamp := float64(cert.NotAfter.Unix())
		ch <- prometheus.MustNewConstMetric(e.cert, prometheus.GaugeValue, timestamp, secretName, namespace)
	}
	e.logger.Log("info", fmt.Sprintf("added secret %s/%s to the metrics", namespace, secretName))

	return nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.cert
}

func New(config Config) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		return nil, err
	}

	// Create k8s api client
	var restConfig *rest.Config
	{
		c := k8srestconfig.Config{
			Logger:    logger,
			InCluster: true,
		}

		restConfig, err = k8srestconfig.New(c)
		if err != nil {
			return nil, err
		}
	}

	var k8sClient *kubernetes.Clientset

	k8sClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	logger.Log("info", "creating new exporter")

	return &Exporter{
		cert: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{
				"name",
				"namespace",
			},
			nil,
		),
		ctx:        ctx,
		k8sClient:  k8sClient,
		logger:     logger,
		namespaces: config.Namespaces,
	}, nil
}
