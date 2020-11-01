package silence

import (
	"context"
	"fmt"
	"regexp"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) EnsureCreated(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	for _, envTarget := range silence.Spec.Targets {
		matcher, err := regexp.Compile(envTarget.Value)
		if err != nil {
			return microerror.Mask(err)
		}

		currentTarget, _ := r.targets[envTarget.Name]
		if !matcher.MatchString(currentTarget) {
			r.logger.LogCtx(ctx, "level", "debug",
				"message", fmt.Sprintf("Silence %#q does not match environment by %#q key [regexp: %#q, value: %#q]",
					silence.Name, envTarget.Name, envTarget.Value, currentTarget))
			r.logger.LogCtx(ctx, "level", "debug", "message", "canceling resource")
			return nil
		}
	}

	return nil
}
