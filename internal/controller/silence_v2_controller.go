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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/config"
	"github.com/giantswarm/silence-operator/pkg/service"
	"github.com/giantswarm/silence-operator/pkg/tenancy"
)

const (
	// FinalizerName is the finalizer added to Silence resources
	FinalizerName = "observability.giantswarm.io/silence-protection"
)

// SilenceV2Reconciler reconciles a Silence object in the observability.giantswarm.io API group
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences/finalizers,verbs=update
type SilenceV2Reconciler struct {
	client client.Client

	silenceService *service.SilenceService
	tenancyHelper  *tenancy.Helper
}

// NewSilenceV2Reconciler creates a new SilenceV2Reconciler with the provided silence service and tenancy helper
func NewSilenceV2Reconciler(client client.Client, silenceService *service.SilenceService, tenancyHelper *tenancy.Helper) *SilenceV2Reconciler {
	return &SilenceV2Reconciler{
		client:         client,
		silenceService: silenceService,
		tenancyHelper:  tenancyHelper,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SilenceV2Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling silence", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished reconciling silence", "namespace", req.Namespace, "name", req.Name)

	silence := &v1alpha2.Silence{}
	err := r.client.Get(ctx, req.NamespacedName, silence)
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
			if err := r.client.Update(ctx, silence); err != nil {
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
		if err := r.client.Update(ctx, silence); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return r.reconcileCreate(ctx, silence)
}

func (r *SilenceV2Reconciler) reconcileCreate(ctx context.Context, silence *v1alpha2.Silence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Convert the Kubernetes CR to alertmanager.Silence
	alertmanagerSilence, err := r.getSilenceFromCR(silence)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Extract tenant information from the silence resource
	tenant := r.tenancyHelper.ExtractTenant(silence)

	logger.Info("Syncing silence with Alertmanager", "tenant", tenant, "namespace", silence.Namespace, "name", silence.Name)

	err = r.silenceService.SyncSilence(ctx, alertmanagerSilence, tenant)
	if err != nil {
		logger.Error(err, "Failed to sync silence with Alertmanager", "tenant", tenant)
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced silence with Alertmanager", "tenant", tenant)
	return ctrl.Result{}, nil
}

func (r *SilenceV2Reconciler) reconcileDelete(ctx context.Context, silence *v1alpha2.Silence) error {
	logger := log.FromContext(ctx)

	// Extract tenant information from the silence resource
	tenant := r.tenancyHelper.ExtractTenant(silence)

	logger.Info("Deleting silence from Alertmanager as part of finalization", "tenant", tenant)

	comment := alertmanager.SilenceComment(silence)
	err := r.silenceService.DeleteSilence(ctx, comment, tenant)
	if err != nil {
		return errors.Wrap(err, "failed to delete silence from Alertmanager")
	}

	logger.Info("Successfully deleted silence from Alertmanager", "tenant", tenant)
	return nil
}

// getSilenceFromCR converts a v1alpha2.Silence to alertmanager.Silence
func (r *SilenceV2Reconciler) getSilenceFromCR(silence *v1alpha2.Silence) (*alertmanager.Silence, error) {
	matchers, err := r.convertMatchers(silence.Spec.Matchers)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	startsAt, endsAt, err := r.calculateSilenceTimes(silence)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	newSilence := &alertmanager.Silence{
		Comment:   alertmanager.SilenceComment(silence),
		CreatedBy: alertmanager.CreatedBy,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Matchers:  matchers,
	}

	return newSilence, nil
}

// convertMatchers converts v1alpha2.SilenceMatcher slice to alertmanager.Matcher slice
func (r *SilenceV2Reconciler) convertMatchers(silenceMatchers []v1alpha2.SilenceMatcher) ([]alertmanager.Matcher, error) {
	var matchers []alertmanager.Matcher
	for _, matcher := range silenceMatchers {
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
		default:
			return nil, errors.Errorf("unsupported match type: %s", matchType)
		}

		matchers = append(matchers, alertmanager.Matcher{
			IsRegex: isRegex,
			IsEqual: isEqual,
			Name:    matcher.Name,
			Value:   matcher.Value,
		})
	}

	return matchers, nil
}

// calculateSilenceTimes implements priority-based time resolution for v1alpha2 silences.
// Priority order:
// 1. EndsAt field (highest priority)
// 2. Duration field
// 3. valid-until annotation (for backward compatibility) + 100-year default fallback
func (r *SilenceV2Reconciler) calculateSilenceTimes(silence *v1alpha2.Silence) (startsAt, endsAt time.Time, err error) {
	now := time.Now()

	// Determine start time: StartsAt field takes precedence over creation timestamp
	if silence.Spec.StartsAt != nil {
		startsAt = silence.Spec.StartsAt.Time
	} else {
		startsAt = silence.GetCreationTimestamp().Time
		// If creation timestamp is zero (shouldn't happen in practice), use current time
		if startsAt.IsZero() {
			startsAt = now
		}
	}

	// Determine end time using priority order
	// Priority 1: EndsAt field (highest priority)
	if silence.Spec.EndsAt != nil {
		endsAt = silence.Spec.EndsAt.Time
		return startsAt, endsAt, nil
	}

	// Priority 2: Duration field
	if silence.Spec.Duration != nil {
		endsAt = startsAt.Add(silence.Spec.Duration.Duration)
		return startsAt, endsAt, nil
	}

	// Priority 3 & 4: Use existing alertmanager.SilenceEndsAt function
	// This handles both valid-until annotation (Priority 3) and 100-year default (Priority 4)
	endsAt, err = alertmanager.SilenceEndsAt(silence)
	if err != nil {
		return time.Time{}, time.Time{}, errors.WithStack(err)
	}
	return startsAt, endsAt, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SilenceV2Reconciler) SetupWithManager(mgr ctrl.Manager, cfg config.Config) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Silence{}).
		Named("silence-v2")

	if cfg.SilenceSelector != nil && !cfg.SilenceSelector.Empty() {
		// Convert labels.Selector to metav1.LabelSelector string representation
		selectorStr := cfg.SilenceSelector.String()
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

	// Add namespace selector predicate if configured
	if cfg.NamespaceSelector != nil && !cfg.NamespaceSelector.Empty() {
		// Create a predicate that filters by namespace labels
		namespacePredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
			namespace := obj.GetNamespace()
			if namespace == "" {
				// Skip cluster-scoped resources
				return false
			}

			// Get the namespace object to check its labels
			ctx := context.Background()
			namespaceObj := &corev1.Namespace{}
			err := mgr.GetClient().Get(ctx, client.ObjectKey{Name: namespace}, namespaceObj)
			if err != nil {
				// If we can't get the namespace, log and skip this object
				ctrl.Log.WithName("silence-v2-controller").Error(err, "Failed to get namespace for namespace selector check", "namespace", namespace)
				return false
			}

			// Check if the namespace matches the selector
			return cfg.NamespaceSelector.Matches(labels.Set(namespaceObj.Labels))
		})
		controllerBuilder = controllerBuilder.WithEventFilter(namespacePredicate)
	}

	return controllerBuilder.Complete(r)
}
