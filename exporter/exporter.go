package exporter

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/afero"
)

type Exporter struct {
	cert   *prometheus.Desc
	fs     afero.Fs
	logger micrologger.Logger
	path   string
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "start collecting metrics")
	ok, err := afero.DirExists(e.fs, e.path)
	if !ok {
		e.logger.Log("error", microerror.Maskf(invalidConfigError, "folder with certs has to exist"))
		return
	}
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return
	}
	err = afero.Walk(e.fs, e.path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			file, err := afero.ReadFile(e.fs, path)
			if err != nil {
				e.logger.Log("error", microerror.Mask(err))
				return err
			}

			block, _ := pem.Decode(file)
      if block == nil {
        return nil
      }
			certs, err := x509.ParseCertificates(block.Bytes)
			if err != nil {
				e.logger.Log("warning", fmt.Sprintf("%s could not be parsed as a certificate: %s", path, microerror.Mask(err)))
				return nil
			}
			if certs == nil {
				return nil
			}

			for _, cert := range certs {
				timestamp := float64(cert.NotAfter.Unix())
				ch <- prometheus.MustNewConstMetric(e.cert, prometheus.GaugeValue, timestamp, path)
			}

		}
		return nil
	})
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
	}
	e.logger.Log("info", "stop collecting metrics")
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.cert
}

func New(path string) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.DefaultConfig())
	if err != nil {
		return nil, err
	}

	fs := afero.NewOsFs()

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
		path:   path,
	}, nil
}
