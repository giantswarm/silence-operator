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

	"github.com/aptible/supercronic/cronexpr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

// RecurringSilenceReconciler reconciles a RecurringSilence object
type RecurringSilenceReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=recurringsilences,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=recurringsilences/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.giantswarm.io,resources=silences,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RecurringSilenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling RecurringSilence")

	var recurringSilence v1alpha1.RecurringSilence
	if err := r.Get(ctx, req.NamespacedName, &recurringSilence); err != nil {
		logger.Error(err, "unable to fetch RecurringSilence")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cron, err := cronexpr.Parse(recurringSilence.Spec.Schedule)
	if err != nil {
		logger.Error(err, "unable to parse schedule")
		return ctrl.Result{}, nil
	}

	now := time.Now()
	nextRun := cron.Next(now)

	if nextRun.IsZero() {
		logger.Info("no next run for this schedule")
		return ctrl.Result{}, nil
	}

	// Create a new Silence object
	silence := &v1alpha1.Silence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recurringSilence.Name,
			Namespace: recurringSilence.Namespace,
		},
		Spec: recurringSilence.Spec.Template,
	}

	if err := ctrl.SetControllerReference(&recurringSilence, silence, r.Scheme()); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, silence); err != nil {
		logger.Error(err, "unable to create Silence for RecurringSilence")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RecurringSilenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RecurringSilence{}).
		Complete(r)
}
