package key

import "github.com/giantswarm/microerror"

var wrongTypeError = &microerror.Error{
	Kind: "wrongTypeError",
}

func IsWrongType(err error) bool {
	return microerror.Cause(err) == wrongTypeError
}

var invalidValidUntilDateError = &microerror.Error{
	Kind: "invalidValidaUntilDateError",
}

func IsInvalidValidUntilDate(err error) bool {
	return microerror.Cause(err) == invalidValidUntilDateError
}

var invalidLocationError = &microerror.Error{
	Kind: "invalidLocationError",
}

func IsInvalidLocation(err error) bool {
	return microerror.Cause(err) == invalidLocationError
}
