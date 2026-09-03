package alertmanager

import (
	"time"

	"github.com/pkg/errors"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

// TODO Can we use open API Types here instead of defining our own types?
type Silence struct {
	Comment   string    `json:"comment"`
	CreatedBy string    `json:"createdBy"`
	EndsAt    time.Time `json:"endsAt"`
	ID        string    `json:"id"`
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	Status    *Status   `json:"status"`
}

type Matcher struct {
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

type Status struct {
	State string `json:"state"`
}

// NewMatcher builds a Matcher from a v1alpha2.MatchType. An empty matchType
// defaults to exact equality (v1alpha2.MatchEqual). It returns an error for any
// other value so callers can fail fast on invalid configuration.
func NewMatcher(matchType v1alpha2.MatchType, name, value string) (Matcher, error) {
	if matchType == "" {
		matchType = v1alpha2.MatchEqual
	}

	var isRegex, isEqual bool
	switch matchType {
	case v1alpha2.MatchEqual:
		isRegex, isEqual = false, true
	case v1alpha2.MatchNotEqual:
		isRegex, isEqual = false, false
	case v1alpha2.MatchRegexMatch:
		isRegex, isEqual = true, true
	case v1alpha2.MatchRegexNotMatch:
		isRegex, isEqual = true, false
	default:
		return Matcher{}, errors.Errorf("invalid match type %q (must be one of %q, %q, %q, %q)", matchType, v1alpha2.MatchEqual, v1alpha2.MatchNotEqual, v1alpha2.MatchRegexMatch, v1alpha2.MatchRegexNotMatch)
	}

	return Matcher{
		IsRegex: isRegex,
		IsEqual: isEqual,
		Name:    name,
		Value:   value,
	}, nil
}
