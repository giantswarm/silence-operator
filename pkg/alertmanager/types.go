package alertmanager

import (
	"time"
)

const (
	// AnnotationStartsAt is the annotation key used to specify the start time of a silence.
	AnnotationStartsAt = "observability.giantswarm.io/starts-at"
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
