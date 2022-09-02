package cr

import (
	"context"
	"fmt"
	"time"

	"github.com/giantswarm/k8sclient/v7/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var certManagerCertificateGroupVersionResource = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Resource: "certificates",
	Version:  "v1",
}

var certManagerIssuerGroupVersionResource = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Resource: "issuers",
	Version:  "v1",
}

var certManagerClusterIssuerGroupVersionResource = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Resource: "clusterissuers",
	Version:  "v1",
}

const (
	issuerLabelSelector = "giantswarm.io/service-type=managed"
	trueString          = "true"
	falseString         = "false"
	unknownString       = ""
)

type Config struct {
	Namespaces []string
}

type Exporter struct {
	certNotAfter  *prometheus.Desc
	ctx           context.Context
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

	namespacesToCheck := []string{""}
	// Create a list of namespaces to check.
	if len(e.namespaces) != 0 {
		namespacesToCheck = e.namespaces
	}

	listOptions := metav1.ListOptions{}

	// Loop over namespaces.
	for _, namespace := range namespacesToCheck {
		certs, err := e.dynamicClient.Resource(certManagerCertificateGroupVersionResource).Namespace(namespace).List(e.ctx, listOptions)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				e.logger.Log("error", microerror.Mask(err))
			}
			continue
		}
		for _, cert := range certs.Items {
			notAfterStatusString, _, err := unstructured.NestedString(cert.UnstructuredContent(), "status", "notAfter")
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
				continue
			}

			notAfter, err := time.Parse(time.RFC3339, notAfterStatusString)
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
				continue
			}
			notAfterUnix := float64(notAfter.Unix())

			issuerRefName, _, err := unstructured.NestedString(cert.UnstructuredContent(), "spec", "issuerRef", "name")
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
			}
			issuerRefKind, _, err := unstructured.NestedString(cert.UnstructuredContent(), "spec", "issuerRef", "kind")
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
			}

			certficateName := cert.GetName()
			certificateNamespace := cert.GetNamespace()
			isManaged := ""

			if issuerRefKind == "ClusterIssuer" {
				isManaged = e.CheckClusterIssuerManaged(issuerRefName)
			} else {
				isManaged = e.CheckIssuerManaged(issuerRefName, certificateNamespace)
			}

			ch <- prometheus.MustNewConstMetric(e.certNotAfter, prometheus.GaugeValue, notAfterUnix, certficateName, certificateNamespace, issuerRefName, isManaged)

			e.logger.Log("info", fmt.Sprintf("added cert-manager certificate CR %s/%s to the metrics", certificateNamespace, certficateName))
		}

	}

	e.logger.Log("info", "finished collecting metrics")
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.certNotAfter
}

func (e *Exporter) CheckIssuerManaged(name, namespace string) string {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
		LabelSelector: issuerLabelSelector,
	}

	issuersList, err := e.dynamicClient.Resource(certManagerIssuerGroupVersionResource).Namespace(namespace).List(e.ctx, listOptions)
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return unknownString
	}

	if len(issuersList.Items) == 1 {
		return trueString
	}

	return falseString
}

func (e *Exporter) CheckClusterIssuerManaged(name string) string {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
		LabelSelector: issuerLabelSelector,
	}

	clusterIssuersList, err := e.dynamicClient.Resource(certManagerClusterIssuerGroupVersionResource).List(e.ctx, listOptions)
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return unknownString
	}

	if len(clusterIssuersList.Items) == 1 {
		return trueString
	}

	return falseString
}

func New(config Config) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		return nil, err
	}

	// Create k8s api client.
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

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	logger.Log("info", "creating new exporter")

	return &Exporter{
		certNotAfter: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "certificate_cr", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{
				"name",
				"namespace",
				"issuer_ref",
				"managed_issuer",
			},
			nil,
		),
		ctx:           ctx,
		dynamicClient: dynClient,
		logger:        logger,
		namespaces:    config.Namespaces,
	}, nil
}
