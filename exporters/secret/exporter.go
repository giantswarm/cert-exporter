package secret

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/giantswarm/k8sclient/v5/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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
	cert          *prometheus.Desc
	ctx           context.Context
	k8sClient     *kubernetes.Clientset
	logger        micrologger.Logger
	dynamicClient dynamic.Interface

	namespaces []string
}

func DefaultConfig() Config {
	return Config{
		Namespaces: []string{},
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "start collecting metrics")

	listOpts := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}

	namespacesToCheck := []string{}
	// Create a list of namespaces to check
	if len(e.namespaces) == 0 {
		namespacesToCheck = append(namespacesToCheck, "")
	} else {
		namespacesToCheck = e.namespaces
	}

	clusterSecrets := []v1.Secret{}
	// Loop over namespaces
	for _, namespace := range namespacesToCheck {
		secrets, err := e.k8sClient.CoreV1().Secrets(namespace).List(e.ctx, listOpts)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}

		clusterSecrets = append(clusterSecrets, secrets.Items...)
	}

	// Loop over discovered secrets
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
	certKeys := [...]string{"ca.crt", "tls.crt"}

	certCRName, err := e.findCertCRName(secretName, secretNamespace)
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
	}

	for _, certKey := range certKeys {
		certBytes, ok := secret.Data[certKey]
		if !ok {
			e.logger.Log("error", microerror.Maskf(certNotFoundError, fmt.Sprintf("secret %s/%s contains no key matching '%s'", secretNamespace, secretName, certKey)))
			continue
		}

		block, _ := pem.Decode(certBytes)
		if block == nil {
			continue
		}

		certs, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			e.logger.Log("warning", fmt.Sprintf("%s in secret %s/%s could not be parsed as a certificate: %s", certKey, secretName, secretNamespace, microerror.Mask(err)))
			continue
		}

		for _, cert := range certs {
			timestamp := float64(cert.NotAfter.Unix())
			issuer := cert.Issuer.String()
			ch <- prometheus.MustNewConstMetric(e.cert, prometheus.GaugeValue, timestamp, secretName, secretNamespace, certKey, issuer, certCRName)
		}
	}
	e.logger.Log("info", fmt.Sprintf("added secret %s/%s to the metrics", secretNamespace, secretName))

	return nil
}

func (e *Exporter) certificateGroupVersionResources() ([]schema.GroupVersionResource, error) {
	groupVersions := []schema.GroupVersionResource{}

	_, resources, err := e.k8sClient.ServerGroupsAndResources()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	for _, rl := range resources {
		for _, r := range rl.APIResources {
			if r.Name == "certificates" && strings.HasPrefix(rl.GroupVersion, "cert-manager.io/") {
				s := schema.FromAPIVersionAndKind(rl.GroupVersion, r.Name)
				groupVersions = append(groupVersions, schema.GroupVersionResource{
					Group:    s.Group,
					Version:  s.Version,
					Resource: r.Name,
				})
			}
		}
	}

	return groupVersions, nil
}

func (e *Exporter) findCertCRName(secretName, secretNamespace string) (string, error) {
	certificateGroupVersionResources, err := e.certificateGroupVersionResources()
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return "", err
	}
	if len(certificateGroupVersionResources) == 0 {
		e.logger.Log("info", "cert-manager Certificate custom resource definition not available, skipping secret collection")
		return "", nil
	}

	for _, gvr := range certificateGroupVersionResources {
		certs, err := e.dynamicClient.Resource(gvr).Namespace(secretNamespace).List(e.ctx, metav1.ListOptions{})
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}
		for _, cert := range certs.Items {
			name, found, err := unstructured.NestedString(cert.UnstructuredContent(), "spec", "secretName")
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
			}
			if found && name == secretName {
				return cert.GetName(), nil
			}
		}
	}

	return "", nil
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

	dynClient, err := dynamic.NewForConfig(restConfig)
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
				"secretkey",
				"issuer",
				"crname",
			},
			nil,
		),
		ctx:           ctx,
		k8sClient:     k8sClient,
		dynamicClient: dynClient,
		logger:        logger,
		namespaces:    config.Namespaces,
	}, nil
}
