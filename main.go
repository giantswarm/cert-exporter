package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/giantswarm/cert-exporter/exporters/cert"
	"github.com/giantswarm/cert-exporter/exporters/token"
	"github.com/giantswarm/cert-exporter/pkg/project"
)

func main() {
	// Print version.
	if (len(os.Args) > 1) && (os.Args[1] == "version") {
		fmt.Printf("Description:    %s\n", project.Description())
		fmt.Printf("Git Commit:     %s\n", project.GitSHA())
		fmt.Printf("Go Version:     %s\n", runtime.Version())
		fmt.Printf("Name:           %s\n", project.Name())
		fmt.Printf("OS / Arch:      %s / %s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Source:         %s\n", project.Source())
		return
	}
	var address string
	var certPaths string
	var tokenPath string
	var vaultURL string
	var help bool
	flag.StringVar(&address, "address", ":9005", "address which cert-exporter uses to listen and serve")
	flag.StringVar(&certPaths, "cert-paths", "", "comma separated folders containing certs to export")
	flag.StringVar(&tokenPath, "token-path", "", "folder containing Vault tokens to export")
	flag.StringVar(&vaultURL, "vault-url", "", "URL of Vault server")
	flag.BoolVar(&help, "help", false, "print usage and exit")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	{
		if certPaths == "" {
			panic(microerror.Maskf(invalidConfigError, "path to cert folder can not be empty"))
		}
		c := cert.DefaultConfig()
		c.Paths = strings.Split(certPaths, ",")

		certExporter, err := cert.New(c)
		if err != nil {
			panic(microerror.Mask(err))
		}
		prometheus.MustRegister(certExporter)
	}

	// Expose Vault token metrics.
	if tokenPath != "" && vaultURL != "" {
		c := token.Config{
			Path:     tokenPath,
			VaultURL: vaultURL,
		}

		tokenExporter, err := token.New(c)
		if err != nil {
			panic(microerror.Mask(err))
		}
		prometheus.MustRegister(tokenExporter)
	}

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(address, nil)
}
