package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/afero"
)

type Config struct {
	Paths []string
}

type Exporter struct {
	cert   *prometheus.Desc
	fs     afero.Fs
	logger micrologger.Logger

	paths []string
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "start collecting metrics")

	// Check every path.
	for _, p := range e.paths {
		err := e.collectPath(ch, p)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
		}
	}

	e.logger.Log("info", "stop collecting metrics")
}

func (e *Exporter) collectPath(ch chan<- prometheus.Metric, path string) error {
	ok, err := afero.DirExists(e.fs, path)
	if !ok {
		e.logger.Log("error", microerror.Maskf(invalidConfigError, fmt.Sprintf("folder %s with certs has to exist", path)))
		return nil
	}
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return nil
	}
	err = afero.Walk(e.fs, path, func(fpath string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			file, err := afero.ReadFile(e.fs, fpath)
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
				return err
			}

			if e.fileIsPrivateKey(file) {
				e.logger.Log("info", fmt.Sprintf("not adding private key %s to the metrics", fpath))
				return nil
			}

			block, _ := pem.Decode(file)
			if block == nil {
				return nil
			}
			certs, err := x509.ParseCertificates(block.Bytes)
			if err != nil {
				e.logger.Log("warning", fmt.Sprintf("%s could not be parsed as a certificate: %s", fpath, microerror.Mask(err)))
				return nil
			}
			if certs == nil {
				return nil
			}

			for _, cert := range certs {
				timestamp := float64(cert.NotAfter.Unix())
				ch <- prometheus.MustNewConstMetric(e.cert, prometheus.GaugeValue, timestamp, fpath)
			}
			e.logger.Log("info", fmt.Sprintf("added %s to the metrics", fpath))

		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// fileIsPrivateKey returns true if the given file contents are an RSA private key, false otherwise.
// As keys don't have expiry date, we don't try to export expiry metrics for them.
func (e *Exporter) fileIsPrivateKey(f []byte) bool {
	return strings.Contains(string(f), "RSA PRIVATE KEY")
}

func DefaultConfig() Config {
	return Config{
		Paths: []string{},
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.cert
}

func New(config Config) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		return nil, err
	}

	fs := afero.NewOsFs()
	logger.Log("info", "creating new exporter")

	return &Exporter{
		cert: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "", "not_after"),
			"Timestamp after which the cert is invalid.",
			[]string{
				"path",
			},
			nil,
		),
		fs:     fs,
		logger: logger,
		paths:  config.Paths,
	}, nil
}
