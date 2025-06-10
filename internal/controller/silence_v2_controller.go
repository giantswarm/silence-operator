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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/service"
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

	silenceService *service.SilenceService
}

// NewSilenceV2Reconciler creates a new SilenceV2Reconciler with the provided silence service
func NewSilenceV2Reconciler(client client.Client, scheme *runtime.Scheme, alertmanager *alertmanager.AlertManager, silenceService *service.SilenceService) *SilenceV2Reconciler {
	return &SilenceV2Reconciler{
		Client:         client,
		Scheme:         scheme,
		Alertmanager:   alertmanager,
		silenceService: silenceService,
	}
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
	}

	return r.reconcileCreate(ctx, silence)
}

func (r *SilenceV2Reconciler) reconcileCreate(ctx context.Context, silence *v1alpha2.Silence) (ctrl.Result, error) {
	// Convert the Kubernetes CR to alertmanager.Silence
	alertmanagerSilence, err := r.getSilenceFromCR(silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	comment := alertmanager.SilenceComment(silence)
	err = r.silenceService.SyncSilence(ctx, comment, alertmanagerSilence)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SilenceV2Reconciler) reconcileDelete(ctx context.Context, silence *v1alpha2.Silence) error {
	comment := alertmanager.SilenceComment(silence)
	return r.silenceService.DeleteSilence(ctx, comment)
}

// getSilenceFromCR converts a v1alpha2.Silence to alertmanager.Silence
func (r *SilenceV2Reconciler) getSilenceFromCR(silence *v1alpha2.Silence) (*alertmanager.Silence, error) {
	var matchers []alertmanager.Matcher
	for _, matcher := range silence.Spec.Matchers {
		// Convert MatchType enum to boolean fields for alertmanager compatibility
		var isRegex, isEqual bool

		// Default to exact match if MatchType is not specified
		matchType := matcher.MatchType
		if matchType == "" {
			matchType = v1alpha2.MatchEqual
		}

		switch matchType {
		case v1alpha2.MatchEqual:
			isRegex = false
			isEqual = true
		case v1alpha2.MatchNotEqual:
			isRegex = false
			isEqual = false
		case v1alpha2.MatchRegexMatch:
			isRegex = true
			isEqual = true
		case v1alpha2.MatchRegexNotMatch:
			isRegex = true
			isEqual = false
		}

		matchers = append(matchers, alertmanager.Matcher{
			IsRegex: isRegex,
			IsEqual: isEqual,
			Name:    matcher.Name,
			Value:   matcher.Value,
		})
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

// SetupWithManager sets up the controller with the Manager.
func (r *SilenceV2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Silence{}).
		Named("silence-v2").
		Complete(r)
}
