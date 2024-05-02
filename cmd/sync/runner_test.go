package sync

import (
	"context"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/silence-operator/api/v1alpha1"
	monitoringv1alpha1 "github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFindYamls(t *testing.T) {
	false := false
	testCases := []struct {
		name           string
		inputFiles     map[string][]byte
		outputSilences []v1alpha1.Silence
	}{
		{
			name: "case 0: two valid files",
			inputFiles: map[string][]byte{
				"silences/silence_bar.yaml": []byte(`
apiVersion: monitoring.giantswarm.io/v1alpha1
kind: Silence
metadata:
  name: foo
  namespace: bar
spec:
  matchers:
  - name: foo
    value: bar
    isEqual: false
    isRegex: false
  targetTags:
  - name: foo
    value: bar
`),
			},
			outputSilences: []v1alpha1.Silence{
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
								IsEqual: &false,
							},
						},
					},
				},
			},
		},
		//{
		//	name: "case 1: one invalid file",
		//	inputFiles: map[string][]byte{
		//		"silences/silence_bar.yaml":    []byte("bar"),
		//		"silences/silence_foo.invalid": []byte("foo"),
		//	},
		//	outputSilences: []string{
		//		"silences/silence_bar.yaml",
		//	},
		//},
		//{
		//	name: "case 2: two invalid file",
		//	inputFiles: map[string][]byte{
		//		"silences/silence_bar.invalid": []byte("bar"),
		//		"silences/silence_foo.invalid": []byte("foo"),
		//	},
		//	outputSilences: nil,
		//},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			////// Fake filesystem
			aFs = afero.Afero{Fs: afero.NewMemMapFs()}
			aFs.MkdirAll("silences", 0755)
			for k, v := range tc.inputFiles {
				aFs.WriteFile(k, v, 0644)
			}

			////// Runner

			flags := &flag{
				Dirs: []string{"silences"},
				Tags: []string{"foo=bar"},
			}
			logger, err := micrologger.New(micrologger.Config{
				IOWriter: io.Discard,
			})
			if err != nil {
				t.Errorf("Error: %v", err)
			}

			r := runner{
				flag:   flags,
				logger: logger,
				stderr: os.Stderr,
				stdout: os.Stdout,
			}
			var nilCmd *cobra.Command = nil
			var ctx = context.Background()

			////// Fake client
			scheme := runtime.NewScheme()
			v1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			////// Run it
			err = r.run(ctx, nilCmd, fakeClient, []string{})
			if err != nil {
				t.Errorf("Error: %v", err)
			}

			var resultSilences monitoringv1alpha1.SilenceList
			{
				err = fakeClient.List(ctx, &resultSilences)
				if err != nil {
					t.Errorf("Error: %v", err)
				}
			}
			for i, resulSilence := range resultSilences.Items {

				if i < len(tc.outputSilences)-1 {
					t.Errorf("Error: Silence not found")
				}

				expectedSilence := tc.outputSilences[i]
				if !cmp.Equal(resulSilence, expectedSilence) {
					t.Fatalf("\n\n%s\n", cmp.Diff(expectedSilence, resulSilence))
				}
			}
		})
	}
}
