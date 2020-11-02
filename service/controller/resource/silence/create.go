package silence

import (
	"context"
	"fmt"
	"regexp"
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

	getOpts := &alertmanager.GetOptions{
		Comment: silence.Name,
	}

	_, err = r.amClient.GetSilence(getOpts)
	if alertmanager.IsNotFound(err) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "creating silence")

		var matchers []alertmanager.Matcher
		{
			for _, matcher := range silence.Spec.Matchers {
				newMatcher := alertmanager.Matcher{
					IsRegex: matcher.IsRegex,
					Name:    matcher.Name,
					Value:   matcher.Value,
				}

				matchers = append(matchers, newMatcher)
			}
		}

		newSilence := &alertmanager.Silence{
			Comment:   silence.Name,
			CreatedBy: createdBy,
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

	r.logger.LogCtx(ctx, "level", "debug", "message", "silence %#q already exists")

	return nil
}
