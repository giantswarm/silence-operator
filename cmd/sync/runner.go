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
	"github.com/giantswarm/k8sclient/v5/pkg/k8srestconfig"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
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

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return microerror.Mask(err)
	}

	var currentSilences v1alpha1.SilenceList
	err = k8sClient.List(ctx, &currentSilences)
	if err != nil {
		return microerror.Mask(err)
	}

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
			tagObj := strings.SplitN(tag, "=", 2)
			tagName := tagObj[0]
			tagValue := ""
			if len(tagObj) == 2 {
				tagValue = tagObj[1]
			}

			tags[tagName] = tagValue
		}
	}

	var filteredSilences []v1alpha1.Silence
	{
		for _, silenceFile := range silenceFiles {
			data, err := ioutil.ReadFile(silenceFile)
			if err != nil {
				return microerror.Mask(err)
			}

			var silence v1alpha1.Silence
			err = yaml.Unmarshal(data, &silence)
			if err != nil {
				return microerror.Mask(err)
			}

			if silenceInList(silence, filteredSilences) {
				r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("skip duplicated silence %#q", silence.Name))
				continue
			}

			// filter target tags
			validSilence := true
			for _, envTag := range silence.Spec.TargetTags {
				matcher, err := regexp.Compile(envTag.Value)
				if err != nil {
					return microerror.Mask(err)
				}

				currentTag := tags[envTag.Name]
				if !matcher.MatchString(currentTag) {
					r.logger.LogCtx(ctx, "level", "debug",
						"message", fmt.Sprintf("silence %#q does not match environment by %#q key [regexp: %#q, value: %#q]",
							silence.Name, envTag.Name, envTag.Value, currentTag))
					validSilence = false
					break
				}
			}

			if validSilence {
				filteredSilences = append(filteredSilences, silence)
			}
		}
	}

	// delete expired silences
	for _, currentSilence := range currentSilences.Items {
		if !silenceInList(currentSilence, filteredSilences) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("deleting expired silence CR %#q", currentSilence.Name))

			err = k8sClient.Delete(ctx, &currentSilence) //nolint:gosec
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("silence %#q has been deleted", currentSilence.Name))
		}
	}

	// create desired silences
	for i, silence := range filteredSilences {
		if !silenceInList(silence, currentSilences.Items) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("creating desired silence CR %#q", silence.Name))

			err = k8sClient.Create(ctx, &filteredSilences[i])
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("desired silence CR %#q has been created", silence.Name))
		}
	}

	return nil
}

func findYamls(dir string) ([]string, error) {
	var result []string

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml") {
			result = append(result, filepath.Join(dir, file.Name()))
		}
	}

	return result, nil
}

func silenceInList(silence v1alpha1.Silence, silences []v1alpha1.Silence) bool {
	for _, item := range silences {
		if item.Name == silence.Name {
			return true
		}
	}

	return false
}
