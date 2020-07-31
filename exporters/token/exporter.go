package token

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
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
	client *vaultapi.Client

	path string
}

// New creates new Exporter object.
func New(config Config) (*Exporter, error) {
	logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		return nil, err
	}

	// Check Vault url is valid.
	_, err = url.ParseRequestURI(config.VaultURL)
	if err != nil {
		return nil, err
	}

	vaultConfig := vaultapi.DefaultConfig()
	vaultConfig.Address = config.VaultURL

	client, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return nil, err
	}

	e := &Exporter{
		metric: prometheus.NewDesc(
			prometheus.BuildFQName("cert_exporter", "token", "not_after"),
			"Timestamp after which the Vault token is expired.",
			[]string{
				"path",
			},
			nil,
		),
		client: client,
		logger: logger,
		path:   config.Path,
	}

	return e, nil
}

// Collect implements metric collection by reading Vault token from files
// and checking ttl of this token by calling Vault API /auth/token/lookup-self.
// Only first line in every file is read. Files expected to contain
// either just a uuid token or VAULT_TOKEN=<token> string.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Log("info", "collecting vault metrics")

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

	tokenRegex, err := regexp.Compile(`.\\..{24}`)
	if err != nil {
		e.logger.Log("error", microerror.Mask(err))
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

		// Make sure token is in expected format.
		if match := tokenRegex.MatchString(token); !match {
			e.logger.Log("error", "bad token format")
			continue
		}

		// Set Vault token.
		e.client.SetToken(token)

		_, err = e.client.Sys().Health()
		if err != nil {
			e.logger.Log("warning", "vault is not healthy")
			continue
		}

		// Get token expiration time.
		expTime, err := e.getTokenExpireTime()
		if IsNoTokenExpiration(err) {
			e.logger.Log("warning", "token has no expiration")
			continue
		} else if err != nil {
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

	e.logger.Log("info", "finished collecting vault metrics")
}

// Describe returns metric metadata.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.metric
}

// getTokenExpireTime makes a lookup-self call to Vault and tries to extract token ttl.
func (e *Exporter) getTokenExpireTime() (float64, error) {
	secret, err := e.client.Auth().Token().LookupSelf()
	if err != nil {
		return 0, microerror.Mask(err)
	}

	key, ok := secret.Data[expireTimeKey]
	if !ok {
		return 0, microerror.Maskf(executionFailedError, "value of '%s' must exist in order to collect metrics for the Vault token expiration", expireTimeKey)
	}

	if key == nil {
		e.logger.Log("level", "info", "message", "Vault token does not expire, skipping metric update")
		return 0, microerror.Mask(noTokenExpirationError)
	}

	s, ok := key.(string)
	if !ok {
		return 0, microerror.Maskf(executionFailedError, "'%#v' must be string in order to collect metrics for the Vault token expiration", key)
	}
	split := strings.Split(s, ".")
	if len(split) == 0 {
		return 0, microerror.Maskf(executionFailedError, "'%#v' must have at least one item in order to collect metrics for the Vault token expiration", s)
	}
	expireTime := split[0]

	t, err := time.Parse(expireTimeLayout, expireTime)
	if err != nil {
		return 0, microerror.Mask(err)
	}

	return float64(t.Unix()), nil
}
