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
	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

const (
	// FinalizerName is the finalizer added to Silence resources
	FinalizerName = "observability.giantswarm.io/silence-protection"
)

// SilenceV2Reconciler reconciles a Silence object in the observability.giantswarm.io API group
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences/finalizers,verbs=update
type SilenceV2Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Alertmanager    *alertmanager.AlertManager
	SilenceSelector labels.Selector
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SilenceV2Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling silence", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished reconciling silence", "namespace", req.Namespace, "name", req.Name)

	silence := &v1alpha2.Silence{}
	err := r.Get(ctx, req.NamespacedName, silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if !silence.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(silence, FinalizerName) {
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
			controllerutil.RemoveFinalizer(silence, FinalizerName)
			if err := r.Update(ctx, silence); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, errors.WithStack(err)
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(silence, FinalizerName) {
		controllerutil.AddFinalizer(silence, FinalizerName)
		if err := r.Update(ctx, silence); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileCreate(ctx, silence)
}

func (r *SilenceV2Reconciler) reconcileCreate(ctx context.Context, silence *v1alpha2.Silence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	newSilence, err := r.getSilenceFromCR(silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	now := time.Now()

	existingSilence, err := r.Alertmanager.GetSilenceByComment(alertmanager.SilenceCommentV1Alpha2(silence))
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

func (r *SilenceV2Reconciler) reconcileDelete(ctx context.Context, silence *v1alpha2.Silence) error {
	logger := log.FromContext(ctx)
	logger.Info("deleting silence")

	err := r.Alertmanager.DeleteSilenceByComment(alertmanager.SilenceCommentV1Alpha2(silence))
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
func (r *SilenceV2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Silence{}).
		Named("silence-v2")

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

func (r *SilenceV2Reconciler) getSilenceFromCR(silence *v1alpha2.Silence) (*alertmanager.Silence, error) {
	var matchers []alertmanager.Matcher
	for _, matcher := range silence.Spec.Matchers {
		isEqual := true
		if matcher.IsEqual != nil {
			isEqual = *matcher.IsEqual
		}

		matchers = append(matchers, alertmanager.Matcher{
			IsRegex: matcher.IsRegex,
			IsEqual: isEqual,
			Name:    matcher.Name,
			Value:   matcher.Value,
		})
	}

	// Convert to v1alpha1 format for compatibility with existing alertmanager functions
	v1Alpha1Silence := convertToV1Alpha1(silence)
	endsAt, err := alertmanager.SilenceEndsAt(v1Alpha1Silence)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	newSilence := &alertmanager.Silence{
		Comment:   alertmanager.SilenceCommentV1Alpha2(silence),
		CreatedBy: alertmanager.CreatedBy,
		StartsAt:  silence.GetCreationTimestamp().Time,
		EndsAt:    endsAt,
		Matchers:  matchers,
	}

	return newSilence, nil
}

// convertToV1Alpha1 converts a v1alpha2.Silence to v1alpha1.Silence for compatibility with existing alertmanager functions
func convertToV1Alpha1(v2Silence *v1alpha2.Silence) *v1alpha1.Silence {
	v1Matchers := make([]v1alpha1.Matcher, len(v2Silence.Spec.Matchers))
	for i, matcher := range v2Silence.Spec.Matchers {
		v1Matchers[i] = v1alpha1.Matcher{
			IsRegex: matcher.IsRegex,
			IsEqual: matcher.IsEqual,
			Name:    matcher.Name,
			Value:   matcher.Value,
		}
	}

	return &v1alpha1.Silence{
		TypeMeta:   v2Silence.TypeMeta,
		ObjectMeta: v2Silence.ObjectMeta,
		Spec: v1alpha1.SilenceSpec{
			Matchers: v1Matchers,
			Owner:    v2Silence.Spec.Owner,
			IssueURL: v2Silence.Spec.IssueURL,
		},
	}
}
