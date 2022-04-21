package sync

import (
	"context"
	"io"
	"testing"

	"github.com/giantswarm/micrologger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

func Test_isValidSilence(t *testing.T) {
	tests := []struct {
		Name    string
		Silence v1alpha1.Silence
		Tags    []string
		IsValid bool
	}{
		{
			"empty silence against no tags is valid",
			v1alpha1.Silence{},
			nil,
			true,
		},
		{
			"empty silence against tags is valid",
			v1alpha1.Silence{},
			[]string{
				"foo=bar",
			},
			true,
		},
		{
			"silence with tags against same tags is valid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			[]string{
				"foo=bar",
			},
			true,
		},
		{
			"silence with tags against different tags is invalid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			[]string{
				"one=two",
			},
			false,
		},
		{
			"silence with tags against no tags is invalid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			nil,
			false,
		},
		{
			"silence with multiple tags against same tags is valid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
							"one": "two",
						},
					},
				},
			},
			[]string{
				"foo=bar",
				"one=two",
			},
			true,
		},
		{
			"silence with multiple unordered tags against same tags is valid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"one": "two",
							"foo": "bar",
						},
					},
				},
			},
			[]string{
				"foo=bar",
				"one=two",
			},
			true,
		},
		{
			"silence with multiple tags against unordered same tags is valid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
							"one": "two",
						},
					},
				},
			},
			[]string{
				"one=two",
				"foo=bar",
			},
			true,
		},
		{
			"silence with multiple tags against one tag is invalid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
							"one": "two",
						},
					},
				},
			},
			[]string{
				"one=two",
			},
			false,
		},
		{
			"silence with one tag against multiple tags is valid",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"one": "two",
						},
					},
				},
			},
			[]string{
				"foo=bar",
				"one=two",
			},
			true,
		},
		{
			"silence weird",
			v1alpha1.Silence{
				Spec: v1alpha1.SilenceSpec{
					TargetTags: []v1alpha1.TargetTag{
						{
							Name:  "one",
							Value: "",
						},
					},
				},
			},
			[]string{
				"foo=bar",
			},
			true,
		},
	}

	logger, err := micrologger.New(micrologger.Config{IOWriter: io.Discard})
	//logger, err := micrologger.New(micrologger.Config{})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			runner := runner{
				logger: logger,
				flag: &flag{
					Tags: test.Tags,
				},
			}

			tags := runner.loadTags()

			isValid, err := runner.isValidSilence(context.Background(), test.Silence, tags)
			if err != nil {
				t.Error(err)
			}
			if isValid != test.IsValid {
				t.Errorf("failure: expected isValid=%t, got isValid=%t with silence.spec.targetTags %v matched with tags %v\n", test.IsValid, isValid, test.Silence.Spec.TargetTags, test.Tags)
			}
		})
	}
}
