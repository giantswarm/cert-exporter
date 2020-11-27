package secret

import (
	"github.com/giantswarm/microerror"
)

var certNotFoundError = &microerror.Error{
	Kind: "certNotFoundError",
}

// NoCertFound asserts cerNotFoundError
func NoCertFound(err error) bool {
	return microerror.Cause(err) == certNotFoundError
}
