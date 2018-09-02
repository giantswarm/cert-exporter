package token

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

// IsExecutionFailed asserts executionFailedError.
func IsExecutionFailed(err error) bool {
	return microerror.Cause(err) == executionFailedError
}

var noTokenExpirationError = &microerror.Error{
	Kind: "noTokenExpirationError",
}

// IsNoTokenExpiration asserts noTokenExpirationError.
func IsNoTokenExpiration(err error) bool {
	return microerror.Cause(err) == noTokenExpirationError
}
