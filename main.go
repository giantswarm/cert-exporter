package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/afero"
)

var (
	description string = "The cert-exporter walks a directory path it has gotten as input and emits all NotAfter timestamps as metrics."
	gitCommit   string = "n/a"
	name        string = "cert-exporter"
	source      string = "https://github.com/giantswarm/cert-exporter"
)

type Exporter struct {
	cert   *prometheus.Desc
	fs     afero.Fs
	logger micrologger.Logger
	path   string
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.cert
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

func main() {
	// Print version.
	if (len(os.Args) > 1) && (os.Args[1] == "version") {
		fmt.Printf("Description:    %s\n", description)
		fmt.Printf("Git Commit:     %s\n", gitCommit)
		fmt.Printf("Go Version:     %s\n", runtime.Version())
		fmt.Printf("Name:           %s\n", name)
		fmt.Printf("OS / Arch:      %s / %s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Source:         %s\n", source)
		return
	}
	var certPath string
	flag.StringVar(&certPath, "path", "", "folder containing certs to export")
	flag.Parse()
	if certPath == "" {
		panic(microerror.Maskf(invalidConfigError, "path to cert folder can not be empty"))
	}

	prometheus.MustRegister(newExporter(certPath))

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe("localhost:8000", nil)
}

func newExporter(path string) *Exporter {
	logger, err := micrologger.New(micrologger.DefaultConfig())
	if err != nil {
		microerror.Mask(err)
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
	}
}
