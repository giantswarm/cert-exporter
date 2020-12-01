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
	certType      = "secret"
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

	var clusterSecrets []v1.Secret

	listOpts := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}

	// Build list of secrets to check
	if len(e.namespaces) == 0 {
		// If no namespaces are provided then get all secrets matching selector
		secrets, err := e.k8sClient.CoreV1().Secrets("").List(e.ctx, listOpts)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}

		for _, secret := range secrets.Items {
			clusterSecrets = append(clusterSecrets, secret)
		}
	} else {
		// get secrets from given namespaces and append to list
		for _, namespace := range e.namespaces {
			// Get secrets in namespace
			secrets, err := e.k8sClient.CoreV1().Secrets(namespace).List(e.ctx, listOpts)
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
			}

			for _, secret := range secrets.Items {
				clusterSecrets = append(clusterSecrets, secret)
			}
		}
	}

	// Range over discovered secrets
	for _, secret := range clusterSecrets {
		err := e.calculateExpiry(ch, secret)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}
	}

	e.logger.Log("info", "finished collecting metrics")
}

func (e *Exporter) calculateExpiry(ch chan<- prometheus.Metric, secret v1.Secret) error {
	secretName := secret.Name
	secretNamespace := secret.Namespace
	certBytes, ok := secret.Data["tls.crt"]
	if !ok {
		e.logger.Log("error", microerror.Maskf(certNotFoundError, fmt.Sprintf("secret %s/%s contains no key matching 'tls.crt'", secretNamespace, secretName)))
		return nil
	}

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
		ch <- prometheus.MustNewConstMetric(e.cert, prometheus.GaugeValue, timestamp, secretName, secretNamespace, certType)
	}
	e.logger.Log("info", fmt.Sprintf("added secret %s/%s to the metrics", secretNamespace, secretName))

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
			prometheus.BuildFQName("cert_exporter", "secret", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{
				"name",
				"namespace",
				"type",
			},
			nil,
		),
		ctx:        ctx,
		k8sClient:  k8sClient,
		logger:     logger,
		namespaces: config.Namespaces,
	}, nil
}
