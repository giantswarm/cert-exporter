package secret

import (
	"github.com/giantswarm/microerror"
)

var certNotFoundError = &microerror.Error{
	Kind: "certNotFoundError",
}

// IsCertNotFound asserts certNotFoundError
func IsCertNotFound(err error) bool {
	return microerror.Cause(err) == certNotFoundError
}
