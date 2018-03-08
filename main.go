package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/giantswarm/cert-exporter/exporter"
	"github.com/giantswarm/cert-exporter/exporters/token"
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
	var address string
	var certPath string
	var tokenPath string
	var vaultURL string
	var help bool
	flag.StringVar(&address, "address", ":9005", "address which cert-exporter uses to listen and serve")
	flag.StringVar(&certPath, "path", "", "folder containing certs to export")
	flag.StringVar(&tokenPath, "token-path", "", "folder containing Vault tokens to export")
	flag.StringVar(&vaultURL, "vault-url", "", "URL of Vault server")
	flag.BoolVar(&help, "help", false, "print usage and exit")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	{
		if certPath == "" {
			panic(microerror.Maskf(invalidConfigError, "path to cert folder can not be empty"))
		}
		config := exporter.DefaultConfig()
		config.Path = certPath

		certExporter, err := exporter.New(config)
		if err != nil {
			panic(microerror.Mask(err))
		}
		prometheus.MustRegister(certExporter)
	}

	// Expose Vault token metrics.
	{
		if tokenPath == "" || vaultURL == "" {
			panic(microerror.Maskf(invalidConfigError, "path to token folder and Vault URL can not be empty"))
		}
		config := token.DefaultConfig()
		config.Path = tokenPath
		config.VaultURL = vaultURL

		tokenExporter, err := token.New(config)
		if err != nil {
			panic(microerror.Mask(err))
		}
		prometheus.MustRegister(tokenExporter)
	}

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(address, nil)
}
