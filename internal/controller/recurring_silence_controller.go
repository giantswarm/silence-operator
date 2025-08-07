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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/aptible/supercronic/cronexpr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

const (
	// RecurringSilenceFinalizerName is the finalizer added to RecurringSilence resources
	RecurringSilenceFinalizerName = "observability.giantswarm.io/recurring-silence-protection"
	
	// ConditionTypeScheduled represents whether the RecurringSilence is properly scheduled
	ConditionTypeScheduled = "Scheduled"
	
	// ConditionReasonCronParseError indicates the cron expression could not be parsed
	ConditionReasonCronParseError = "CronParseError"
	
	// ConditionReasonDurationParseError indicates the duration could not be parsed
	ConditionReasonDurationParseError = "DurationParseError"
	
	// ConditionReasonScheduled indicates the RecurringSilence is properly scheduled
	ConditionReasonScheduled = "Scheduled"
	
	// ConditionReasonSilenceCreateError indicates an error creating a silence
	ConditionReasonSilenceCreateError = "SilenceCreateError"
)

// RecurringSilenceReconciler reconciles a RecurringSilence object
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences/finalizers,verbs=update
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
type RecurringSilenceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewRecurringSilenceReconciler creates a new RecurringSilenceReconciler
func NewRecurringSilenceReconciler(client client.Client, scheme *runtime.Scheme) *RecurringSilenceReconciler {
	return &RecurringSilenceReconciler{
		Client: client,
		Scheme: scheme,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RecurringSilenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling recurring silence", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished reconciling recurring silence", "namespace", req.Namespace, "name", req.Name)

	recurringSilence := &v1alpha2.RecurringSilence{}
	err := r.Get(ctx, req.NamespacedName, recurringSilence)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !recurringSilence.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(recurringSilence, RecurringSilenceFinalizerName) {
			if err := r.reconcileDelete(ctx, recurringSilence); err != nil {
				logger.Error(err, "Failed to delete associated silences during finalization")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(recurringSilence, RecurringSilenceFinalizerName)
			if err := r.Update(ctx, recurringSilence); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(recurringSilence, RecurringSilenceFinalizerName) {
		controllerutil.AddFinalizer(recurringSilence, RecurringSilenceFinalizerName)
		if err := r.Update(ctx, recurringSilence); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the recurring silence
	return r.reconcileRecurringSilence(ctx, recurringSilence)
}

// reconcileRecurringSilence handles the main reconciliation logic
func (r *RecurringSilenceReconciler) reconcileRecurringSilence(ctx context.Context, recurringSilence *v1alpha2.RecurringSilence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	
	// Parse cron expression
	cronExpr, err := cronexpr.Parse(recurringSilence.Spec.Schedule)
	if err != nil {
		logger.Error(err, "Failed to parse cron expression", "schedule", recurringSilence.Spec.Schedule)
		r.setCondition(recurringSilence, ConditionTypeScheduled, metav1.ConditionFalse, ConditionReasonCronParseError, err.Error())
		if updateErr := r.Status().Update(ctx, recurringSilence); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	// Parse duration
	duration, err := time.ParseDuration(recurringSilence.Spec.Duration)
	if err != nil {
		logger.Error(err, "Failed to parse duration", "duration", recurringSilence.Spec.Duration)
		r.setCondition(recurringSilence, ConditionTypeScheduled, metav1.ConditionFalse, ConditionReasonDurationParseError, err.Error())
		if updateErr := r.Status().Update(ctx, recurringSilence); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	now := time.Now()
	
	// Check if we need to create a silence
	shouldCreateSilence := false
	lastScheduled := recurringSilence.Status.LastScheduledTime
	
	if lastScheduled == nil {
		// First time - check if we're past a scheduled time
		// Look back up to the duration period to see if we missed a trigger
		lookBackTime := now.Add(-duration)
		if nextTime := cronExpr.Next(lookBackTime); !nextTime.IsZero() && nextTime.Before(now) {
			shouldCreateSilence = true
		}
	} else {
		// Check if it's time for the next scheduled silence
		nextTime := cronExpr.Next(lastScheduled.Time)
		if !nextTime.IsZero() && nextTime.Before(now) {
			shouldCreateSilence = true
		}
	}

	// Create or update silence if needed
	if shouldCreateSilence {
		silenceName := fmt.Sprintf("%s-silence", recurringSilence.Name)
		if err := r.createOrUpdateSilence(ctx, recurringSilence, silenceName, now, duration); err != nil {
			logger.Error(err, "Failed to create or update silence")
			r.setCondition(recurringSilence, ConditionTypeScheduled, metav1.ConditionFalse, ConditionReasonSilenceCreateError, err.Error())
			if updateErr := r.Status().Update(ctx, recurringSilence); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}

		// Update status
		recurringSilence.Status.LastScheduledTime = &metav1.Time{Time: now}
		recurringSilence.Status.ActiveSilence = &silenceName
	}

	// Calculate next scheduled time
	nextTime := cronExpr.Next(now)
	if !nextTime.IsZero() {
		recurringSilence.Status.NextScheduledTime = &metav1.Time{Time: nextTime}
	}

	// Set condition to scheduled
	r.setCondition(recurringSilence, ConditionTypeScheduled, metav1.ConditionTrue, ConditionReasonScheduled, "RecurringSilence is properly scheduled")

	// Update status
	if err := r.Status().Update(ctx, recurringSilence); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Calculate requeue time - check again at the next scheduled time or in 1 minute, whichever is sooner
	var requeueAfter time.Duration
	if !nextTime.IsZero() {
		requeueAfter = time.Until(nextTime)
		if requeueAfter > time.Hour {
			requeueAfter = time.Hour // Don't wait more than an hour
		}
	} else {
		requeueAfter = time.Hour // No more scheduled times, check again in an hour
	}

	if requeueAfter < time.Minute {
		requeueAfter = time.Minute // Don't requeue too frequently
	}

	logger.Info("Requeuing recurring silence", "requeueAfter", requeueAfter.String(), "nextScheduledTime", nextTime)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// createOrUpdateSilence creates or updates a silence resource
func (r *RecurringSilenceReconciler) createOrUpdateSilence(ctx context.Context, recurringSilence *v1alpha2.RecurringSilence, silenceName string, startTime time.Time, duration time.Duration) error {
	silence := &v1alpha2.Silence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      silenceName,
			Namespace: recurringSilence.Namespace,
		},
		Spec: v1alpha2.SilenceSpec{
			Matchers: recurringSilence.Spec.Matchers,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(recurringSilence, silence, r.Scheme); err != nil {
		return errors.Wrap(err, "failed to set controller reference")
	}

	// Try to get existing silence
	existingSilence := &v1alpha2.Silence{}
	err := r.Get(ctx, types.NamespacedName{Name: silenceName, Namespace: recurringSilence.Namespace}, existingSilence)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get existing silence")
	}

	if apierrors.IsNotFound(err) {
		// Create new silence
		if err := r.Create(ctx, silence); err != nil {
			return errors.Wrap(err, "failed to create silence")
		}
		log.FromContext(ctx).Info("Created new silence", "silence", silenceName)
	} else {
		// Update existing silence if needed
		existingSilence.Spec.Matchers = recurringSilence.Spec.Matchers
		if err := r.Update(ctx, existingSilence); err != nil {
			return errors.Wrap(err, "failed to update silence")
		}
		log.FromContext(ctx).Info("Updated existing silence", "silence", silenceName)
	}

	return nil
}

// reconcileDelete handles the deletion of a RecurringSilence
func (r *RecurringSilenceReconciler) reconcileDelete(ctx context.Context, recurringSilence *v1alpha2.RecurringSilence) error {
	logger := log.FromContext(ctx)

	// Delete associated silences
	silenceList := &v1alpha2.SilenceList{}
	if err := r.List(ctx, silenceList, client.InNamespace(recurringSilence.Namespace), client.MatchingFields{"metadata.ownerReferences.uid": string(recurringSilence.UID)}); err != nil {
		return errors.Wrap(err, "failed to list associated silences")
	}

	for _, silence := range silenceList.Items {
		if err := r.Delete(ctx, &silence); err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete silence %s", silence.Name)
		}
		logger.Info("Deleted associated silence", "silence", silence.Name)
	}

	return nil
}

// setCondition sets a condition on the RecurringSilence status
func (r *RecurringSilenceReconciler) setCondition(recurringSilence *v1alpha2.RecurringSilence, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&recurringSilence.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RecurringSilenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.RecurringSilence{}).
		Named("recurring-silence").
		Complete(r)
}