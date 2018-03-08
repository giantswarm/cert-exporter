package token

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = microerror.New("execution failed")
var invalidConfigError = microerror.New("invalid config")

// IsExecutionFailed asserts executionFailedError.
func IsExecutionFailed(err error) bool {
	return microerror.Cause(err) == executionFailedError
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}
