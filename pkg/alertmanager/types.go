package alertmanager

import (
	"time"

	"github.com/pkg/errors"
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

// Match type strings understood by NewMatcher. These mirror the values of
// v1alpha2.MatchType so that both CR conversion and the enforcement config
// loader can share a single conversion routine.
const (
	MatchTypeEqual         = "="
	MatchTypeNotEqual      = "!="
	MatchTypeRegexMatch    = "=~"
	MatchTypeRegexNotMatch = "!~"
)

// NewMatcher builds a Matcher from a match-type string (one of "=", "!=", "=~",
// "!~"). An empty matchType defaults to exact equality ("="). It returns an
// error for any other value so callers can fail fast on invalid configuration.
func NewMatcher(matchType, name, value string) (Matcher, error) {
	if matchType == "" {
		matchType = MatchTypeEqual
	}

	var isRegex, isEqual bool
	switch matchType {
	case MatchTypeEqual:
		isRegex, isEqual = false, true
	case MatchTypeNotEqual:
		isRegex, isEqual = false, false
	case MatchTypeRegexMatch:
		isRegex, isEqual = true, true
	case MatchTypeRegexNotMatch:
		isRegex, isEqual = true, false
	default:
		return Matcher{}, errors.Errorf("invalid match type %q (must be one of %q, %q, %q, %q)", matchType, MatchTypeEqual, MatchTypeNotEqual, MatchTypeRegexMatch, MatchTypeRegexNotMatch)
	}

	return Matcher{
		IsRegex: isRegex,
		IsEqual: isEqual,
		Name:    name,
		Value:   value,
	}, nil
}
