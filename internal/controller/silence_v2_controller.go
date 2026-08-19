/*
Copyright 2026.

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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/config"
	"github.com/giantswarm/silence-operator/pkg/enforce"
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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
type SilenceV2Reconciler struct {
	client client.Client

	silenceService *service.SilenceService
	tenancyHelper  *tenancy.Helper
	// enforcer applies namespace-scoped silence enforcement. It is nil when no
	// enforcement config is provided, in which case enforcement is skipped.
	enforcer *enforce.Enforcer
	// recorder emits Kubernetes Events on the Silence resource (e.g. when an
	// enforced matcher overrides a user-supplied one). May be nil.
	recorder record.EventRecorder
}

// NewSilenceV2Reconciler creates a new SilenceV2Reconciler with the provided
// silence service, tenancy helper, enforcer (may be nil) and event recorder
// (may be nil).
func NewSilenceV2Reconciler(client client.Client, silenceService *service.SilenceService, tenancyHelper *tenancy.Helper, enforcer *enforce.Enforcer, recorder record.EventRecorder) *SilenceV2Reconciler {
	return &SilenceV2Reconciler{
		client:         client,
		silenceService: silenceService,
		tenancyHelper:  tenancyHelper,
		enforcer:       enforcer,
		recorder:       recorder,
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

	// Apply namespace-scoped enforcement (inject the namespace matcher and any
	// configured custom matchers) when a rule matches the silence's namespace.
	if err := r.applyEnforcement(ctx, silence, alertmanagerSilence); err != nil {
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
	matchers, err := convertMatchers(silence.Spec.Matchers)
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

func convertMatchers(silenceMatchers []v1alpha2.SilenceMatcher) ([]alertmanager.Matcher, error) {
	var matchers []alertmanager.Matcher
	for _, matcher := range silenceMatchers {
		// Convert the CR MatchType enum into alertmanager's boolean fields.
		amMatcher, err := alertmanager.NewMatcher(matcher.MatchType, matcher.Name, matcher.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		matchers = append(matchers, amMatcher)
	}

	return matchers, nil
}

// calculateSilenceTimes resolves start and end times using the following priority chain:
//  1. spec.startsAt / spec.endsAt (explicit timestamps)
//  2. spec.startsAt + spec.duration
//  3. creationTimestamp + valid-until annotation (migration path from v1alpha1)
//  4. creationTimestamp + 100-year default (v1alpha1 backward compatibility)
func (r *SilenceV2Reconciler) calculateSilenceTimes(silence *v1alpha2.Silence) (startsAt, endsAt time.Time, err error) {
	if silence.Spec.StartsAt != nil {
		startsAt = silence.Spec.StartsAt.Time
	} else {
		startsAt = silence.GetCreationTimestamp().Time
		if startsAt.IsZero() {
			return time.Time{}, time.Time{}, errors.New("creationTimestamp is zero")
		}
	}

	if silence.Spec.EndsAt != nil {
		return startsAt, silence.Spec.EndsAt.Time, nil
	}

	if silence.Spec.Duration != nil {
		d, err := silence.Spec.Duration.Duration()
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return startsAt, startsAt.Add(d), nil
	}

	// Fall back to valid-until annotation, then 100-year default.
	endsAt, err = alertmanager.SilenceEndsAt(silence)
	if err != nil {
		return time.Time{}, time.Time{}, errors.WithStack(err)
	}
	return startsAt, endsAt, nil
}

// applyEnforcement injects the first matching rule's enforced matchers (and,
// when a namespaceMatcherLabel is configured, an authoritative namespace
// matcher) into the Alertmanager silence when the silence's namespace matches an
// enforcement rule. It is a no-op when no enforcer is configured. When
// enforcement overrides a user-supplied matcher, a warning is logged and a
// Kubernetes Event is emitted on the Silence so the owner can see why the
// silence was modified.
func (r *SilenceV2Reconciler) applyEnforcement(ctx context.Context, silence *v1alpha2.Silence, alertmanagerSilence *alertmanager.Silence) error {
	if r.enforcer == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	// Fetch the namespace to read its labels for selector matching.
	namespaceObj := &corev1.Namespace{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: silence.Namespace}, namespaceObj); err != nil {
		return errors.Wrapf(err, "failed to get namespace %q for enforcement", silence.Namespace)
	}

	enforced, matched := r.enforcer.MatchersFor(silence.Namespace, namespaceObj.Labels)
	if !matched {
		return nil
	}

	replaced := enforce.ApplyEnforcedMatchers(alertmanagerSilence, enforced)
	if len(replaced) > 0 {
		logger.Info("Enforcement overrode user-supplied matchers",
			"namespace", silence.Namespace, "name", silence.Name, "overriddenMatchers", replaced)
		if r.recorder != nil {
			r.recorder.Eventf(silence, corev1.EventTypeWarning, "MatcherOverridden",
				"Enforcement overrode matcher(s) %v with enforced values", replaced)
		}
	}

	return nil
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
