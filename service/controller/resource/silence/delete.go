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

	deleteOpts := &alertmanager.DeleteOptions{
		Comment: silence.Name,
	}

	silenceID := ""
	err = r.amClient.DeleteSilence(silenceID, deleteOpts)
	if err != nil {
		return microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "silence has been deleted")

	return nil
}
