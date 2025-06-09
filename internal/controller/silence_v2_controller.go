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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

	SilenceService *service.SilenceService

	SilenceSelector   labels.Selector
	NamespaceSelector labels.Selector
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

	// Convert the Silence object to an Alertmanager silence
	alertmanagerSilence, err := convertV1alpha2(silence)
	if err != nil {
		logger.Error(err, "Failed to convert Silence to Alertmanager silence")
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !silence.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(silence, FinalizerName) {
			// Our finalizer is present, so let's handle external dependency deletion
			if err := r.SilenceService.DeleteSilence(ctx, alertmanagerSilence); err != nil {
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

	// Use the service for reconciliation
	changed, err := r.SilenceService.SyncSilence(ctx, alertmanagerSilence)
	if err != nil {
		logger.Error(err, "Failed to sync silence with Alertmanager")
		return ctrl.Result{}, errors.WithStack(err)
	}

	if changed {
		logger.Info("Successfully updated silence in Alertmanager")
	} else {
		logger.Info("No changes needed for silence")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SilenceV2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.Silence{}).
		Named("silence-v2")

	// Add silence selector predicate if configured
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

	// Add namespace selector predicate if configured
	if r.NamespaceSelector != nil && !r.NamespaceSelector.Empty() {
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
			return r.NamespaceSelector.Matches(labels.Set(namespaceObj.Labels))
		})
		controllerBuilder = controllerBuilder.WithEventFilter(namespacePredicate)
	}

	return controllerBuilder.Complete(r)
}

func convertV1alpha2(silence *v1alpha2.Silence) (*alertmanager.Silence, error) {
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
