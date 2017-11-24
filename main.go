package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/giantswarm/cert-exporter/exporter"
	"github.com/giantswarm/microerror"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	description string = "The cert-exporter walks a directory path it has gotten as input and emits all NotAfter timestamps as metrics."
	gitCommit   string = "n/a"
	name        string = "cert-exporter"
	source      string = "https://github.com/giantswarm/cert-exporter"
)

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
	certExporter, err := exporter.New(certPath)
	if err != nil {
		panic(microerror.Mask(err))
	}
	prometheus.MustRegister(certExporter)

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe("localhost:8000", nil)
}
