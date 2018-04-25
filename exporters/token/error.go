package token

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = microerror.New("execution failed")

// IsExecutionFailed asserts executionFailedError.
func IsExecutionFailed(err error) bool {
	return microerror.Cause(err) == executionFailedError
}

var noTokenExpirationError = microerror.New("no token expiration")

// IsNoTokenExpiration asserts noTokenExpirationError.
func IsNoTokenExpiration(err error) bool {
	return microerror.Cause(err) == noTokenExpirationError
}
