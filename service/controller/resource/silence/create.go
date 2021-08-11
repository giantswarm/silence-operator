package silence

import (
	"context"
	"time"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) EnsureCreated(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	_, err = r.amClient.GetSilenceByComment(key.SilenceComment(silence))
	if alertmanager.IsNotFound(err) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "creating silence")

		var matchers []alertmanager.Matcher
		{
			for _, matcher := range silence.Spec.Matchers {
				newMatcher := alertmanager.Matcher{
					IsEqual: matcher.IsEqual,
					IsRegex: matcher.IsRegex,
					Name:    matcher.Name,
					Value:   matcher.Value,
				}
				matchers = append(matchers, newMatcher)
			}
		}

		newSilence := &alertmanager.Silence{
			Comment:   key.SilenceComment(silence),
			CreatedBy: key.CreatedBy,
			EndsAt:    eternity,
			Matchers:  matchers,
			StartsAt:  time.Now(),
		}

		err = r.amClient.CreateSilence(newSilence)
		if err != nil {
			return microerror.Mask(err)
		}
	}
	if err != nil {
		return microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "silence already exists")

	return nil
}
