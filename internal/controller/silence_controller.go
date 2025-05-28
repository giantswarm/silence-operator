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

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// Define the finalizer name
const silenceFinalizer = "monitoring.giantswarm.io/silence-protection"
const legacySilenceFinalizer = "operatorkit.giantswarm.io/silence-operator-silence-controller"

// SilenceReconciler reconciles a Silence object
type SilenceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Alertmanager    *alertmanager.AlertManager
	SilenceSelector labels.Selector
}

// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SilenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling silence")
	defer logger.Info("Finished reconciling silence")

	silence := &v1alpha1.Silence{}
	err := r.Get(ctx, req.NamespacedName, silence)
	if err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	// Handle deletion: The object is being deleted
	if !silence.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(silence, silenceFinalizer) {
			// Our finalizer is present, so let's handle external dependency deletion
			if err := r.reconcileDelete(ctx, silence); err != nil {
				// If fail to delete the external dependency here, return error
				// so that it can be retried.
				logger.Error(err, "Failed to delete Alertmanager silence during finalization")
				return ctrl.Result{}, err
			}

			// Once the external dependency is deleted, remove the finalizer.
			// This allows the Kubernetes API server to finalize the object deletion.
			logger.Info("Removing finalizer after successful Alertmanager silence deletion")
			controllerutil.RemoveFinalizer(silence, silenceFinalizer)
			if err := r.Update(ctx, silence); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, errors.WithStack(err)
			}
		}

		// If the legacy finalizer is present, remove it after the new finalizer has been removed.
		if err = r.cleanUpLegacyFinalizer(ctx, silence); err != nil {
			logger.Error(err, "Failed to remove legacy finalizer")
			return ctrl.Result{}, errors.WithStack(err)
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is present: The object is not being deleted
	if !controllerutil.ContainsFinalizer(silence, silenceFinalizer) {
		logger.Info("Adding finalizer")
		controllerutil.AddFinalizer(silence, silenceFinalizer)
		if err := r.Update(ctx, silence); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// If the legacy finalizer is present, remove it after the new finalizer has been added.
	if err = r.cleanUpLegacyFinalizer(ctx, silence); err != nil {
		logger.Error(err, "Failed to remove legacy finalizer")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Reconcile the creation/update of the silence
	return r.reconcileCreate(ctx, silence)
}

func (r *SilenceReconciler) cleanUpLegacyFinalizer(ctx context.Context, silence *v1alpha1.Silence) error {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(silence, legacySilenceFinalizer) {
		logger.Info("Removing legacy finalizer")
		controllerutil.RemoveFinalizer(silence, legacySilenceFinalizer)
		if err := r.Update(ctx, silence); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
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
	if err != nil && !errors.Is(err, alertmanager.ErrSilenceNotFound) {
		logger.Error(err, "Failed to get silence from Alertmanager")
		return ctrl.Result{}, errors.WithStack(err)
	} else if errors.Is(err, alertmanager.ErrSilenceNotFound) {
		if newSilence.EndsAt.After(now) {
			logger.Info("Creating silence in Alertmanager")
			err = r.Alertmanager.CreateSilence(newSilence)
			if err != nil {
				logger.Error(err, "Failed to create silence in Alertmanager")
				return ctrl.Result{}, errors.WithStack(err)
			}
			logger.Info("Created silence in Alertmanager")
		} else {
			logger.Info("Skipped creation: silence is already expired")
		}
	} else if newSilence.EndsAt.Before(now) {
		// Existing silence found, but the desired state is expired
		logger.Info("Deleting expired silence from Alertmanager")
		err = r.Alertmanager.DeleteSilenceByID(existingSilence.ID)
		if err != nil {
			logger.Error(err, "Failed to delete expired silence from Alertmanager")
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("Deleted expired silence from Alertmanager")
	} else if updateNeeded(existingSilence, newSilence) {
		newSilence.ID = existingSilence.ID
		logger.Info("Updating silence in Alertmanager")
		err = r.Alertmanager.UpdateSilence(newSilence)
		if err != nil {
			logger.Error(err, "Failed to update silence in Alertmanager")
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("Updated silence in Alertmanager")
	} else {
		logger.Info("Skipped update: silence unchanged")
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion of the external Alertmanager silence.
func (r *SilenceReconciler) reconcileDelete(ctx context.Context, silence *v1alpha1.Silence) error {
	logger := log.FromContext(ctx)
	logger.Info("Deleting silence from Alertmanager as part of finalization")

	err := r.Alertmanager.DeleteSilenceByComment(alertmanager.SilenceComment(silence))
	if err != nil {
		// If the silence is already gone in Alertmanager, treat it as success
		if errors.Is(err, alertmanager.ErrSilenceNotFound) {
			logger.Info("Silence already deleted in Alertmanager")
			return nil // Success, allows finalizer removal
		}
		// For other errors, return the error to retry
		return errors.Wrap(err, "failed to delete silence from Alertmanager")
	}

	logger.Info("Successfully deleted silence from Alertmanager")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SilenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Silence{}).
		Named("silence")

	if r.SilenceSelector != nil && !r.SilenceSelector.Empty() {
		// Convert labels.Selector to metav1.LabelSelector string representation
		selectorStr := r.SilenceSelector.String()
		// Parse the string into metav1.LabelSelector
		metaLabelSelector, err := metav1.ParseToLabelSelector(selectorStr)
		if err != nil {
			return errors.Wrap(err, "failed to parse silence selector for predicate")
		}
		// Create the predicate using controller-runtime's LabelSelectorPredicate
		labelPredicate, err := predicate.LabelSelectorPredicate(*metaLabelSelector)
		if err != nil {
			return errors.Wrap(err, "failed to create label selector predicate")
		}
		controllerBuilder = controllerBuilder.WithEventFilter(labelPredicate)
	}

	return controllerBuilder.Complete(r)
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
		return nil, errors.WithStack(err)
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
