package sync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/silence-operator/api/v1alpha1"
	monitoringv1alpha1 "github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFindYamls(t *testing.T) {
	// falseValue is needed in order to create a pointer to false. Stupid but still needed.
	falseValue := false

	// testCases holds an array of each test case to be run.
	testCases := []struct {
		name             string
		inputSilences    []v1alpha1.Silence
		expectedSilences []v1alpha1.Silence
	}{
		{
			name: "case 0: two valid files",
			inputSilences: []v1alpha1.Silence{
				v1alpha1.Silence{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
					Spec: v1alpha1.SilenceSpec{
						TargetTags: []v1alpha1.TargetTag{
							{
								Name:  "foo",
								Value: "bar",
							},
						},
						Matchers: []v1alpha1.Matcher{
							{
								Name:  "foo",
								Value: "bar",
							},
						},
					},
				},
			},
			expectedSilences: []v1alpha1.Silence{
				v1alpha1.Silence{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "monitoring.giantswarm.io/v1alpha1",
						Kind:       "Silence",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "foo",
						Namespace:       "bar",
						ResourceVersion: "1",
					},
					Spec: v1alpha1.SilenceSpec{
						TargetTags: []v1alpha1.TargetTag{
							{
								Name:  "foo",
								Value: "bar",
							},
						},
						Matchers: []v1alpha1.Matcher{
							{
								Name:    "foo",
								Value:   "bar",
								IsEqual: &falseValue,
							},
						},
					},
				},
			},
		},
	}

	// Run each test case.
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			// Initialize a fake filesystem in memory with inputSilences converted as yaml files.
			aFs = afero.Afero{Fs: afero.NewMemMapFs()}
			aFs.MkdirAll("silences", 0755)
			for i, silence := range tc.inputSilences {
				v, err := yaml.Marshal(silence)
				if err != nil {
					t.Fatalf("Failed to marshall silence %d: %v", i, err)
				}
				filename := filepath.Join("silences", silence.GetName()+".yaml")
				aFs.WriteFile(filename, v, 0644)
			}

			// Initialize a fake k8s client capable of handling Silences.
			scheme := runtime.NewScheme()
			v1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			// Initialize sync command runner.
			flags := &flag{
				Dirs: []string{"silences"},
				Tags: []string{"foo=bar"},
			}
			logger, err := micrologger.New(micrologger.Config{
				IOWriter: io.Discard,
			})
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			r := runner{
				flag:   flags,
				logger: logger,
				stderr: os.Stderr,
				stdout: os.Stdout,
			}

			// Run the runner
			var nilCmd *cobra.Command = nil
			var ctx = context.Background()
			var args = []string{}
			err = r.run(ctx, nilCmd, fakeClient, args)
			if err != nil {
				t.Fatalf("Failed to run runner: %v", err)
			}

			// Check resulting silences against expectedSilences.
			var resultSilences monitoringv1alpha1.SilenceList
			{
				err = fakeClient.List(ctx, &resultSilences)
				if err != nil {
					t.Fatalf("Failed to get resulting silences: %v", err)
				}
			}
			for i, expectedSilence := range tc.expectedSilences {
				if !silenceInList(expectedSilence, resultSilences.Items) {
					t.Errorf("Error: Silence %d not found in list: %v", i, expectedSilence)
				}
			}
		})
	}
}
