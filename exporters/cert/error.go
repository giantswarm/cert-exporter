package cert

import (
	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var certsPathNotFoundError = &microerror.Error{
	Kind: "certsPathNotFound",
}

// IsCertsPathNotFound asserts certsPathNotFoundError.
func IsCertsPathNotFound(err error) bool {
	return microerror.Cause(err) == certsPathNotFoundError
}
