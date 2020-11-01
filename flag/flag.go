package flag

import (
	"github.com/giantswarm/microkit/flag"

	"github.com/giantswarm/template-operator/flag/service"
)

// Flag provides data structure for service command line flags.
type Flag struct {
	Service service.Service
}

// New constructs fills new Flag structure with given command line flags.
func New() *Flag {
	f := &Flag{}
	flag.Init(f)

	return f
}
