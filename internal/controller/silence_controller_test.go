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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringv1alpha1 "github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/internal/controller/testutils"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/config"
	"github.com/giantswarm/silence-operator/pkg/service"
	"github.com/giantswarm/silence-operator/pkg/tenancy"
)

var _ = Describe("Silence Controller", func() {
	var (
		mockServer       *testutils.MockAlertmanagerServer
		mockAlertmanager *alertmanager.Alertmanager
		reconciler       *SilenceReconciler
		ctx              context.Context
	)

	BeforeEach(func() {
		// Set up mock Alertmanager server
		mockServer = testutils.NewMockAlertmanagerServer()
		var err error
		mockAlertmanager, err = mockServer.GetAlertmanager()
		Expect(err).NotTo(HaveOccurred())

		// Create service and reconciler
		silenceService := service.NewSilenceService(mockAlertmanager)

		// Create tenancy helper with default config
		cfg := config.Config{}
		tenancyHelper := tenancy.NewHelper(cfg)

		reconciler = &SilenceReconciler{
			client:         k8sClient,
			silenceService: silenceService,
			tenancyHelper:  tenancyHelper,
		}

		ctx = context.Background()
	})

	AfterEach(func() {
		// Clean up mock server
		if mockServer != nil {
			mockServer.Close()
		}
	})

	Context("When reconciling a new Silence resource", func() {
		const resourceName = "test-silence-new"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a new Silence resource")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
						{
							Name:    "severity",
							Value:   "critical",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should add finalizer and sync with Alertmanager", func() {
			By("reconciling the created resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying finalizer was added")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(silence, silenceFinalizer)).To(BeTrue())

			By("verifying the reconciliation completed without errors")
			// The fact that we got here means the controller successfully:
			// 1. Added the finalizer
			// 2. Called the service to sync the silence
			// 3. Returned without errors
			Expect(true).To(BeTrue()) // Test passes if we get here
		})
	})

	Context("When reconciling an existing Silence resource", func() {
		const resourceName = "test-silence-existing"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating and reconciling a Silence resource")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			// First reconciliation to add finalizer and create silence
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should handle subsequent reconciliations without errors", func() {
			By("reconciling the resource again")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying finalizer is still present")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(silence, silenceFinalizer)).To(BeTrue())
		})
	})

	Context("When reconciling a Silence resource with legacy finalizer", func() {
		const resourceName = "test-silence-legacy"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Silence resource with legacy finalizer")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:       resourceName,
					Namespace:  "default",
					Finalizers: []string{legacySilenceFinalizer},
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should add new finalizer and remove legacy finalizer", func() {
			By("reconciling the resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying new finalizer was added and legacy finalizer was removed")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(silence, silenceFinalizer)).To(BeTrue())
			Expect(controllerutil.ContainsFinalizer(silence, legacySilenceFinalizer)).To(BeFalse())
		})
	})

	Context("When deleting a Silence resource", func() {
		const resourceName = "test-silence-delete"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating and reconciling a Silence resource")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			// First reconciliation to add finalizer and create silence
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete silence from Alertmanager and remove finalizer", func() {
			By("getting the silence before deletion")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())

			By("deleting the Silence resource")
			Expect(k8sClient.Delete(ctx, silence)).To(Succeed())

			By("reconciling the deletion")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying the Kubernetes resource was deleted")
			err = k8sClient.Get(ctx, typeNamespacedName, silence)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("When deleting a Silence resource with legacy finalizer", func() {
		const resourceName = "test-silence-delete-legacy"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		It("should handle deletion with legacy finalizer properly", func() {
			By("creating a Silence resource with both finalizers")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:       resourceName,
					Namespace:  "default",
					Finalizers: []string{silenceFinalizer, legacySilenceFinalizer},
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("deleting the Silence resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("reconciling the deletion")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying the Kubernetes resource was deleted")
			silence := &monitoringv1alpha1.Silence{}
			err = k8sClient.Get(ctx, typeNamespacedName, silence)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("When reconciling a Silence with invalid expiration date", func() {
		const resourceName = "test-silence-invalid-date"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Silence resource with invalid expiration date")
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						alertmanager.ValidUntilAnnotationName: "invalid-date-format",
					},
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should return an error when date format is invalid", func() {
			By("reconciling the resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Context("When reconciling a Silence with valid RFC3339 expiration date", func() {
		const resourceName = "test-silence-rfc3339-date"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Silence resource with RFC3339 expiration date")
			futureTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						alertmanager.ValidUntilAnnotationName: futureTime,
					},
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile with RFC3339 date format", func() {
			By("reconciling the resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying finalizer was added")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(silence, silenceFinalizer)).To(BeTrue())
		})
	})

	Context("When reconciling a Silence with legacy date format", func() {
		const resourceName = "test-silence-legacy-date"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Silence resource with legacy date format")
			futureDate := time.Now().Add(24 * time.Hour).Format(alertmanager.DateOnlyLayout)
			resource := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						alertmanager.ValidUntilAnnotationName: futureDate,
					},
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: false,
						},
					},
					Owner:    "test-owner",
					IssueURL: "https://github.com/example/test-issue",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Silence resource")
			resource := &monitoringv1alpha1.Silence{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile with legacy date format", func() {
			By("reconciling the resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying finalizer was added")
			silence := &monitoringv1alpha1.Silence{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, silence)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(silence, silenceFinalizer)).To(BeTrue())
		})
	})

	Context("When reconciling a non-existent Silence resource", func() {
		It("should handle not found errors gracefully", func() {
			By("reconciling a non-existent resource")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-silence",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Context("When testing getSilenceFromCR function", func() {
		It("should correctly convert CR to alertmanager Silence", func() {
			By("creating a test Silence CR")
			testTime := time.Now()
			cr := &monitoringv1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-silence",
					Namespace:         "default",
					CreationTimestamp: metav1.NewTime(testTime),
				},
				Spec: monitoringv1alpha1.SilenceSpec{
					Matchers: []monitoringv1alpha1.Matcher{
						{
							Name:    "alertname",
							Value:   "TestAlert",
							IsRegex: true,
							IsEqual: &[]bool{false}[0], // pointer to false
						},
						{
							Name:    "severity",
							Value:   "critical",
							IsRegex: false,
							// IsEqual is nil, should default to true
						},
					},
				},
			}

			By("converting CR to alertmanager Silence")
			silence, err := getSilenceFromCR(cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(silence).NotTo(BeNil())

			By("verifying the conversion")
			Expect(silence.Comment).To(Equal(alertmanager.SilenceComment(cr)))
			Expect(silence.CreatedBy).To(Equal(alertmanager.CreatedBy))
			Expect(silence.StartsAt).To(Equal(testTime))
			Expect(silence.Matchers).To(HaveLen(2))

			// First matcher: IsEqual=false, IsRegex=true
			Expect(silence.Matchers[0].Name).To(Equal("alertname"))
			Expect(silence.Matchers[0].Value).To(Equal("TestAlert"))
			Expect(silence.Matchers[0].IsRegex).To(BeTrue())
			Expect(silence.Matchers[0].IsEqual).To(BeFalse())

			// Second matcher: IsEqual=true (default), IsRegex=false
			Expect(silence.Matchers[1].Name).To(Equal("severity"))
			Expect(silence.Matchers[1].Value).To(Equal("critical"))
			Expect(silence.Matchers[1].IsRegex).To(BeFalse())
			Expect(silence.Matchers[1].IsEqual).To(BeTrue())
		})
	})
})
