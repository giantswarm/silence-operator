package silence

import (
	"context"
	"fmt"
	"regexp"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) EnsureDeleted(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	for _, envTag := range silence.Spec.TargetTags {
		matcher, err := regexp.Compile(envTag.Value)
		if err != nil {
			return microerror.Mask(err)
		}

		currentTag, _ := r.tags[envTag.Name]
		if !matcher.MatchString(currentTag) {
			r.logger.LogCtx(ctx, "level", "debug",
				"message", fmt.Sprintf("silence %#q does not match environment by %#q key [regexp: %#q, value: %#q]",
					silence.Name, envTag.Name, envTag.Value, currentTag))
			r.logger.LogCtx(ctx, "level", "debug", "message", "canceling resource")
			return nil
		}
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "deleting silence")

	deleteOpts := &alertmanager.DeleteOptions{
		Comment: silence.Name,
	}

	err = r.amClient.DeleteSilence(deleteOpts)
	if err != nil {
		return microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "silence has been deleted")

	return nil
}
