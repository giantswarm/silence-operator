/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/microerror"
	"github.com/pkg/errors"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// SilenceReconciler reconciles a Silence object
type SilenceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Alertmanager *alertmanager.AlertManager
}

// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *SilenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling silence")
	defer logger.Info("Finished reconciling silence")

	silence := &v1alpha1.Silence{}
	err := r.Get(ctx, req.NamespacedName, silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if !silence.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, silence)
	}

	// TODO add finalizer to silence CRs to prevent deletion of silences

	return r.reconcileCreate(ctx, silence)
}

// TODO encapsulate business logic in a separate package
func (r *SilenceReconciler) reconcileCreate(ctx context.Context, silence *v1alpha1.Silence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	newSilence, err := getSilenceFromCR(silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	now := time.Now()

	var existingSilence *alertmanager.Silence
	existingSilence, err = r.Alertmanager.GetSilenceByComment(alertmanager.SilenceComment(silence))
	notFound := alertmanager.IsNotFound(err)
	if !notFound && err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	if notFound {
		if newSilence.EndsAt.After(now) {
			logger.Info("creating silence")

			err = r.Alertmanager.CreateSilence(newSilence)
			if err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
			logger.Info("created silence")
		} else {
			logger.Info("skipped creation : silence is expired")
		}
	} else if newSilence.EndsAt.Before(now) {
		logger.Info("deleting silence")

		err = r.Alertmanager.DeleteSilenceByID(existingSilence.ID)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("deleted silence")
	} else if updateNeeded(existingSilence, newSilence) {
		newSilence.ID = existingSilence.ID
		logger.Info("updating silence")

		err = r.Alertmanager.UpdateSilence(newSilence)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("updated silence")
	} else {
		logger.Info("skipped update : silence unchanged")
	}

	return ctrl.Result{}, nil
}

func (r *SilenceReconciler) reconcileDelete(ctx context.Context, silence *v1alpha1.Silence) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting silence")

	err := r.Alertmanager.DeleteSilenceByComment(alertmanager.SilenceComment(silence))
	if err != nil {
		if alertmanager.IsNotFound(err) {
			logger.Info("silence does not exist")
			return nil
		}
		return errors.WithStack(err)
	}

	logger.Info("silence has been deleted")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SilenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Silence{}).
		Named("silence").
		Complete(r)
}

func getSilenceFromCR(silence *v1alpha1.Silence) (*alertmanager.Silence, error) {
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

	endsAt, err := alertmanager.SilenceEndsAt(silence)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	newSilence := &alertmanager.Silence{
		Comment:   alertmanager.SilenceComment(silence),
		CreatedBy: alertmanager.CreatedBy,
		StartsAt:  silence.GetCreationTimestamp().Time,
		EndsAt:    endsAt,
		Matchers:  matchers,
	}

	return newSilence, nil
}

// updateNeeded return true when silence need to be updated.
func updateNeeded(existingSilence, newSilence *alertmanager.Silence) bool {
	return !reflect.DeepEqual(existingSilence.Matchers, newSilence.Matchers) ||
		!existingSilence.EndsAt.Equal(newSilence.EndsAt)
}
