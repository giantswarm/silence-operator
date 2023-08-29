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

func (r *Resource) getSilenceFromCR(silence v1alpha1.Silence) (*alertmanager.Silence, error) {
	var matchers []alertmanager.Matcher
	{
		for _, matcher := range silence.Spec.Matchers {
			isEqual := true
			if matcher.IsEqual != nil {
				isEqual = *matcher.IsEqual
			}
			newMatcher := alertmanager.Matcher{
				IsEqual: isEqual,
				IsRegex: matcher.IsRegex,
				Name:    matcher.Name,
				Value:   matcher.Value,
			}
			matchers = append(matchers, newMatcher)
		}
	}

	endsAt, err := key.SilenceEndsAt(silence)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	newSilence := &alertmanager.Silence{
		Comment:   key.SilenceComment(silence),
		CreatedBy: key.CreatedBy,
		StartsAt:  silence.GetCreationTimestamp().Time,
		EndsAt:    endsAt,
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
	if !notFound && err != nil {
		return microerror.Mask(err)
	}
	if notFound {
		if newSilence.EndsAt.After(now) {
			r.logger.LogCtx(ctx, "level", "debug", "message", "creating silence")

			err = r.amClient.CreateSilence(newSilence)
			if err != nil {
				return microerror.Mask(err)
			}
			r.logger.LogCtx(ctx, "level", "debug", "message", "created silence")
		} else {
			r.logger.LogCtx(ctx, "level", "debug", "message", "skipped creation : silence is expired")
		}
	} else if newSilence.EndsAt.Before(now) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "deleting silence")

		err = r.amClient.DeleteSilenceByID(existingSilence.ID)
		if err != nil {
			return microerror.Mask(err)
		}
		r.logger.LogCtx(ctx, "level", "debug", "message", "deleted silence")
	} else if updateNeeded(existingSilence, newSilence) {
		newSilence.ID = existingSilence.ID
		r.logger.LogCtx(ctx, "level", "debug", "message", "updating silence")

		err = r.amClient.UpdateSilence(newSilence)
		if err != nil {
			return microerror.Mask(err)
		}
		r.logger.LogCtx(ctx, "level", "debug", "message", "updated silence")
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "skipped update : silence unchanged")
	}

	return nil
}

// updateNeeded return true when silence need to be updated.
func updateNeeded(existingSilence, newSilence *alertmanager.Silence) bool {
	return !cmp.Equal(existingSilence.Matchers, newSilence.Matchers) ||
		!existingSilence.EndsAt.Equal(newSilence.EndsAt)
}
