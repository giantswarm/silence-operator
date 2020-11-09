package silence

import (
	"context"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

func (r *Resource) EnsureDeleted(ctx context.Context, obj interface{}) error {
	silence, err := key.ToSilence(obj)
	if err != nil {
		return microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "deleting silence")

	err = r.amClient.DeleteSilenceByComment(key.SilenceComment(silence))
	if alertmanager.IsNotFound(err) {
		r.logger.LogCtx(ctx, "level", "debug", "message", "silence does not exist")
		return nil
	}
	if err != nil {
		return microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "silence has been deleted")

	return nil
}
