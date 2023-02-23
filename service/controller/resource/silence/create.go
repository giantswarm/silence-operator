package silence

import (
	"context"
	"fmt"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/google/go-cmp/cmp"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) getSilenceFromCR(silence v1alpha1.Silence) (*alertmanager.Silence, error) {
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

	validUntil, err := key.SilenceValidUntil(silence)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	newSilence := &alertmanager.Silence{
		Comment:   key.SilenceComment(silence),
		CreatedBy: key.CreatedBy,
		StartsAt:  time.Now(),
		EndsAt:    validUntil,
		Matchers:  matchers,
	}

	return newSilence, nil
}

func (r *Resource) EnsureCreated(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	newSilence, err := r.getSilenceFromCR(silence)
	if err != nil {
		return microerror.Mask(err)
	}

	now := time.Now()

	var existingSilence *alertmanager.Silence
	existingSilence, err = r.amClient.GetSilenceByComment(key.SilenceComment(silence))
	notFound := alertmanager.IsNotFound(err)
	if notFound && newSilence.EndsAt.After(now) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "creating silence")

		err = r.amClient.CreateSilence(newSilence)
		if err != nil {
			return microerror.Mask(err)
		}
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence created")
	} else if updateNeeded(existingSilence, newSilence) {
		if newSilence.EndsAt.Before(now) {
			r.logger.LogCtx(ctx, "level", "debug", "message", "deleting silence")

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("%+#v", newSilence))
			err = r.amClient.DeleteSilenceByID(existingSilence.ID)
			if err != nil {
				return microerror.Mask(err)
			}
			r.logger.LogCtx(ctx, "level", "debug", "message", "silence deleted")
		} else {
			newSilence.ID = existingSilence.ID
			r.logger.LogCtx(ctx, "level", "debug", "message", "updating silence")

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("%+#v", newSilence))
			err = r.amClient.UpdateSilence(newSilence)
			if err != nil {
				return microerror.Mask(err)
			}
			r.logger.LogCtx(ctx, "level", "debug", "message", "silence updated")
		}
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence already exists")
	}

	return nil
}

// updateNeeded return true when silence need to be updated.
func updateNeeded(existingSilence, newSilence *alertmanager.Silence) bool {
	oneDay := 24 * time.Hour

	return !cmp.Equal(existingSilence.Matchers, newSilence.Matchers) ||
		!existingSilence.EndsAt.Truncate(oneDay).Equal(newSilence.EndsAt.Truncate(oneDay))
}
