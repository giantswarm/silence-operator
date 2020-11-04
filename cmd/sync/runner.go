package sync

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	monitoringv1alpha1 "github.com/giantswarm/apiextensions/v3/pkg/apis/monitoring/v1alpha1"
	monitoringv1alpha1client "github.com/giantswarm/apiextensions/v3/pkg/clientset/versioned/typed/monitoring/v1alpha1"
	"github.com/giantswarm/k8sclient/v4/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type runner struct {
	flag   *flag
	logger micrologger.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	err := r.flag.Validate()
	if err != nil {
		return microerror.Mask(err)
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	var err error

	// find yamls with CRs
	var silenceFiles []string
	{
		for _, dir := range r.flag.Dirs {
			files, err := findYamls(dir)
			if err != nil {
				return microerror.Mask(err)
			}
			silenceFiles = append(silenceFiles, files...)
		}
	}

	tags := make(map[string]string)
	{
		for _, tag := range r.flag.Tags {
			tagObj := strings.Split(tag, "=")
			tagName := tagObj[0]
			tagValue := ""
			if len(tagObj) == 2 {
				tagValue = tagObj[1]
			}

			tags[tagName] = tagValue
		}
	}

	var desiredSilences []monitoringv1alpha1.Silence
	{
		for _, silenceFile := range silenceFiles {
			data, err := ioutil.ReadFile(silenceFile)
			if err != nil {
				return microerror.Mask(err)
			}

			var desiredSilence monitoringv1alpha1.Silence
			err = yaml.Unmarshal(data, &desiredSilence)
			if err != nil {
				return microerror.Mask(err)
			}

			// filter target tags
			var validSilence bool
			for _, envTag := range desiredSilence.Spec.TargetTags {
				matcher, err := regexp.Compile(envTag.Value)
				if err != nil {
					return microerror.Mask(err)
				}

				currentTag, _ := tags[envTag.Name]
				if !matcher.MatchString(currentTag) {
					r.logger.LogCtx(ctx, "level", "debug",
						"message", fmt.Sprintf("silence %#q does not match environment by %#q key [regexp: %#q, value: %#q]",
							desiredSilence.Name, envTag.Name, envTag.Value, currentTag))
					continue
				}
				validSilence = true
			}

			if validSilence {
				desiredSilences = append(desiredSilences, desiredSilence)
			}
		}
	}

	var restConfig *rest.Config
	{
		c := k8srestconfig.Config{
			Logger: r.logger,

			InCluster:  r.flag.KubernetesInCluster,
			KubeConfig: r.flag.KubernetesKubeconfig,
		}

		restConfig, err = k8srestconfig.New(c)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	k8sClient, err := monitoringv1alpha1client.NewForConfig(restConfig)
	if err != nil {
		return microerror.Mask(err)
	}

	currentSilences, err := k8sClient.Silences().List(ctx, metav1.ListOptions{})
	if err != nil {
		return microerror.Mask(err)
	}

	// delete expired silences
	for _, currentSilence := range currentSilences.Items {
		if !silenceInList(currentSilence, desiredSilences) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("deleting expired silence CR %#q", currentSilence.Name))

			err = k8sClient.Silences().Delete(ctx, currentSilence.Name, metav1.DeleteOptions{})
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("silence %#q has been deleted", currentSilence.Name))
		}
	}

	// create desired silences
	for _, desiredSilence := range desiredSilences {
		if !silenceInList(desiredSilence, currentSilences.Items) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("creating desired silence CR %#q", desiredSilence.Name))

			_, err = k8sClient.Silences().Create(ctx, &desiredSilence, metav1.CreateOptions{})
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("desired silence CR %#q has been created", desiredSilence.Name))
		}
	}

	// cleanup alertmanager silences, created by silence-operator, but which don't have silence CR

	return nil
}

func findYamls(dir string) ([]string, error) {
	var result []string

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	for _, file := range files {
		matched, err := filepath.Match("*.yaml", filepath.Base(file.Name()))
		if err != nil {
			return nil, microerror.Mask(err)
		}
		if matched {
			result = append(result, filepath.Join(dir, file.Name()))
		}
	}

	return result, nil
}

func silenceInList(silence monitoringv1alpha1.Silence, silences []monitoringv1alpha1.Silence) bool {
	for _, item := range silences {
		if item.Name == silence.Name {
			return true
		}
	}

	return false
}
