package key

import "github.com/giantswarm/microerror"

var wrongTypeError = &microerror.Error{
	Kind: "wrongTypeError",
}

func IsWrongType(err error) bool {
	return microerror.Cause(err) == wrongTypeError
}

var invalidExpirationDateError = &microerror.Error{
	Kind: "invalidExpirationDateError",
}

func IsInvalidExpirationDate(err error) bool {
	return microerror.Cause(err) == invalidExpirationDateError
}
