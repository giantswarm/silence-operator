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

	"github.com/aptible/supercronic/cronexpr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

const (
	// RecurringSilenceFinalizerName is the finalizer added to RecurringSilence resources.
	RecurringSilenceFinalizerName = "observability.giantswarm.io/recurring-silence-protection"
)

// RecurringSilenceReconciler reconciles a RecurringSilence object.
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=recurringsilences/finalizers,verbs=update
// +kubebuilder:rbac:groups=observability.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
type RecurringSilenceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewRecurringSilenceReconciler creates a new RecurringSilenceReconciler.
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

	var rs v1alpha2.RecurringSilence
	if err := r.Get(ctx, req.NamespacedName, &rs); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("RecurringSilence resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RecurringSilence")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Handle deletion
	if !rs.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &rs)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&rs, RecurringSilenceFinalizerName) {
		controllerutil.AddFinalizer(&rs, RecurringSilenceFinalizerName)
		if err := r.Update(ctx, &rs); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return r.reconcile(ctx, &rs)
}

func (r *RecurringSilenceReconciler) reconcile(ctx context.Context, rs *v1alpha2.RecurringSilence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Parse schedule
	schedule, err := cronexpr.Parse(rs.Spec.Schedule)
	if err != nil {
		logger.Error(err, "Invalid cron schedule", "schedule", rs.Spec.Schedule)
		// Do not requeue, the resource needs to be fixed by the user.
		return ctrl.Result{}, nil
	}

	now := time.Now()
	lastScheduleTime := rs.Status.LastScheduleTime
	if lastScheduleTime == nil {
		// If the last schedule time is not set, use the creation timestamp.
		lastScheduleTime = &metav1.Time{Time: rs.ObjectMeta.CreationTimestamp.Time}
	}

	// Get the next scheduled run time.
	nextScheduleTime := schedule.Next(lastScheduleTime.Time)

	// If the next schedule is in the future, and we are not in a test, requeue for later.
	if nextScheduleTime.After(now) && now.Sub(lastScheduleTime.Time) < (time.Minute) {
		requeueAfter := nextScheduleTime.Sub(now)
		logger.Info("Next silence creation is scheduled", "at", nextScheduleTime, "requeuing after", requeueAfter)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Create silence
	if err := r.createSilence(ctx, rs, nextScheduleTime); err != nil {
		logger.Error(err, "Failed to create silence")
		return ctrl.Result{}, err
	}

	// Update status
	rs.Status.LastScheduleTime = &metav1.Time{Time: nextScheduleTime}
	if err := r.Status().Update(ctx, rs); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Requeue to create the next silence
	return ctrl.Result{Requeue: true}, nil
}

func (r *RecurringSilenceReconciler) createSilence(ctx context.Context, rs *v1alpha2.RecurringSilence, scheduledTime time.Time) error {
	logger := log.FromContext(ctx)

	silence := &v1alpha2.Silence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", rs.Name, scheduledTime.Unix()),
			Namespace: rs.Namespace,
			Annotations: map[string]string{
				alertmanager.AnnotationStartsAt: scheduledTime.Format(time.RFC3339),
			},
		},
		Spec: rs.Spec.SilenceTemplate,
	}

	if err := controllerutil.SetControllerReference(rs, silence, r.Scheme); err != nil {
		return errors.WithStack(err)
	}

	logger.Info("Creating new silence", "silenceName", silence.Name)
	if err := r.Create(ctx, silence); err != nil {
		return errors.WithStack(err)
	}

	// Update status with the new silence reference
	ref, err := reference.GetReference(r.Scheme, silence)
	if err != nil {
		return errors.WithStack(err)
	}
	rs.Status.Active = append(rs.Status.Active, *ref)

	return nil
}

func (r *RecurringSilenceReconciler) reconcileDelete(ctx context.Context, rs *v1alpha2.RecurringSilence) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling deletion of RecurringSilence")

	if controllerutil.ContainsFinalizer(rs, RecurringSilenceFinalizerName) {
		// Clean up associated silences
		var silences v1alpha2.SilenceList
		if err := r.List(ctx, &silences, client.InNamespace(rs.Namespace)); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		for _, silence := range silences.Items {
			if metav1.IsControlledBy(&silence, rs) {
				logger.Info("Deleting associated silence", "silence", silence.Name)
				if err := r.Delete(ctx, &silence); err != nil {
					return ctrl.Result{}, errors.WithStack(err)
				}
			}
		}

		controllerutil.RemoveFinalizer(rs, RecurringSilenceFinalizerName)
		if err := r.Update(ctx, rs); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RecurringSilenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha2.RecurringSilence{}).
		Owns(&v1alpha2.Silence{}).
		Complete(r)
}
