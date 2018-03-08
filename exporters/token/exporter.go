package token

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/google/uuid"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	filePrefix       = "VAULT_TOKEN="
	expireTimeKey    = "expire_time"
	expireTimeLayout = "2006-01-02T15:04:05"
)

// Config implements configuration for token exporter.
type Config struct {
	Path     string
	VaultURL string
}

// Exporter implements metrics exporter for Vault tokens.
type Exporter struct {
	metric *prometheus.Desc
	logger micrologger.Logger

	path     string
	vaultURL string
}

// Collect implements metric collection by reading Vault token from files
// and cheking ttl of this token by calling Vault API /auth/token/lookup-self.
// Only first line in every file is read. Files expected to contain
// either just a uuid token or VAULT_TOKEN=<token> string.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "start collecting metrics")

	files, err := ioutil.ReadDir(e.path)
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
		return
	}

	// Skip if no files.
	if len(files) == 0 {
		e.logger.Log("info", fmt.Sprintf("%s is empty, skipping", e.path))
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fpath := filepath.Join(e.path, file.Name())

		f, err := os.Open(fpath)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
			continue
		}
		defer f.Close()

		// Read only the first line.
		s := bufio.NewScanner(f)
		s.Split(bufio.ScanLines)
		s.Scan()

		// Get token by removing prefix from line.
		token := strings.TrimPrefix(s.Text(), filePrefix)

		// Make sure we did not hit any errors while reading the file.
		if err := s.Err(); err != nil {
			e.logger.Log("error", microerror.Mask(err))
			continue
		}

		// Make sure token is in uuid format.
		_, err = uuid.Parse(token)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
			continue
		}

		// Get Vault client.
		client, err := initVaultClient(e.vaultURL, token)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))
			continue
		}

		// Get token expiration time.
		expTime, err := getTokenExpireTime(client)
		if err != nil {
			e.logger.Log("error", microerror.Mask(err))

			// Handle corner cases, when token already expired or there are some other Vault issues.
			// If unable to get real expiration time, set it to 0 (equal to 1970-01-01T00:00:00).
			ch <- prometheus.MustNewConstMetric(e.metric, prometheus.GaugeValue, 0, fpath)
			e.logger.Log("info", fmt.Sprintf("added %s to the metrics", fpath))
			continue
		}

		// Finally update metric with expiration time.
		ch <- prometheus.MustNewConstMetric(e.metric, prometheus.GaugeValue, expTime, fpath)
		e.logger.Log("info", fmt.Sprintf("added %s to the metrics", fpath))
	}

	e.logger.Log("info", "stop collecting metrics")
}

func initVaultClient(vaultURL, token string) (*vaultapi.Client, error) {
	// Check Vault url is valid.
	_, err := url.ParseRequestURI(vaultURL)
	if err != nil {
		return nil, err
	}

	config := vaultapi.DefaultConfig()
	config.Address = vaultURL

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	client.SetToken(token)

	return client, nil
}

// Make a call lookup-self call to Vault and try to extract token ttl.
func getTokenExpireTime(c *vaultapi.Client) (float64, error) {
	secret, err := c.Auth().Token().LookupSelf()
	if err != nil {
		return 0, microerror.Mask(err)
	}

	key, ok := secret.Data[expireTimeKey]
	if !ok {
		return 0, microerror.Maskf(executionFailedError, "failed to get '%s'", expireTimeKey)
	}
	e, ok := key.(string)
	if !ok {
		return 0, microerror.Maskf(executionFailedError, "failed to convert to string '%#v'", e)
	}
	split := strings.Split(e, ".")
	if len(split) == 0 {
		return 0, microerror.Maskf(executionFailedError, "failed to parse '%#v'", split)
	}
	expireTime := split[0]

	t, err := time.Parse(expireTimeLayout, expireTime)
	if err != nil {
		return 0, microerror.Mask(err)
	}

	return float64(t.Unix()), nil
}

// DefaultConfig creates default configuration.
func DefaultConfig() Config {
	return Config{
		Path:     "",
		VaultURL: "",
	}
}

// Describe returns metric metadata.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.metric
}

// New creates new Exporter object.
func New(config Config) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &Exporter{
		metric: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "token", "not_after"),
			"Timestamp after which the Vault token is expired.",
			[]string{
				"path",
			},
			nil,
		),
		logger:   logger,
		path:     config.Path,
		vaultURL: config.VaultURL,
	}, nil
}
