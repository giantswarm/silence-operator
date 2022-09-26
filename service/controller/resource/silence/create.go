package silence

import (
	"context"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/google/go-cmp/cmp"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) getSilenceFromCR(silence v1alpha1.Silence) *alertmanager.Silence {
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

	return newSilence
}

func (r *Resource) EnsureCreated(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	newSilence := r.getSilenceFromCR(silence)

	var existingSilence *alertmanager.Silence
	existingSilence, err = r.amClient.GetSilenceByComment(key.SilenceComment(silence))
	notFound := alertmanager.IsNotFound(err)
	if notFound {
		r.logger.LogCtx(ctx, "level", "debug", "message", "creating silence")

		err = r.amClient.CreateSilence(newSilence)
		if err != nil {
			return microerror.Mask(err)
		}
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence created")
	} else if !cmp.Equal(existingSilence.Matchers, newSilence.Matchers) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "updating silence")

		newSilence.ID = existingSilence.ID
		err = r.amClient.UpdateSilence(newSilence)
		if err != nil {
			return microerror.Mask(err)
		}
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence updated")
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence already exists")
	}

	return nil
}
