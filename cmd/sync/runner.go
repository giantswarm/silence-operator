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
	"github.com/giantswarm/k8sclient/v6/pkg/k8sclient"
	"github.com/giantswarm/k8sclient/v6/pkg/k8srestconfig"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1alpha1 "github.com/giantswarm/silence-operator/api/v1alpha1"
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

	var ctrlClient client.Client
	{
		c := k8srestconfig.Config{
			Logger: r.logger,

			InCluster:  r.flag.KubernetesInCluster,
			KubeConfig: r.flag.KubernetesKubeconfig,
		}
		ctrlClient, err = getCtrlClient(c)
	}

	{
		if err != nil {
			return microerror.Mask(err)
		}

	var currentSilences monitoringv1alpha1.SilenceList
	err = ctrlClient.List(ctx, &currentSilences)
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

	var filteredSilences []monitoringv1alpha1.Silence
	{
		for _, silenceFile := range silenceFiles {
			data, err := ioutil.ReadFile(silenceFile)
			if err != nil {
				return microerror.Mask(err)
			}

			var silence monitoringv1alpha1.Silence
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
		if !silenceInList(currentSilence, filteredSilences) && !hasKeepAnnotation(currentSilence) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("deleting expired silence CR %#q", currentSilence.Name))

			err = ctrlClient.Delete(ctx, &currentSilence) //nolint:gosec
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

			err = ctrlClient.Create(ctx, &filteredSilences[i])
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("desired silence CR %#q has been created", silence.Name))
		}
	}

	return nil
}

func getCtrlClient(config k8srestconfig.Config) (client.Client, error) {
	restConfig, err := k8srestconfig.New(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	c := k8sclient.ClientsConfig{
		Logger:     config.Logger,
		RestConfig: restConfig,
		SchemeBuilder: k8sclient.SchemeBuilder{
			monitoringv1alpha1.AddToScheme,
		},
	}

	k8sClients, err := k8sclient.NewClients(c)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return k8sClients.CtrlClient(), nil
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

func silenceInList(silence monitoringv1alpha1.Silence, silences []monitoringv1alpha1.Silence) bool {
	for _, item := range silences {
		if item.Name == silence.Name {
			return true
		}
	}

	return false
}

func hasKeepAnnotation(silence monitoringv1alpha1.Silence) bool {
	keep, ok := silence.ObjectMeta.Annotations[annotation.Keep]
	return ok && keep == "true"
}
