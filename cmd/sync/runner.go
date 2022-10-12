package sync

import (
	"context"
	"fmt"
	"io"
	"os"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
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

	// Create kubernetes client.
	var ctrlClient client.Client
	{
		ctrlClient, err = r.getCtrlClient()
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// Load current silences from kubernetes.
	var currentSilences monitoringv1alpha1.SilenceList
	{
		err = ctrlClient.List(ctx, &currentSilences)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	// Load silence filtering tags.
	var tags = make(map[string]string)
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

	// Load desired silences from files.
	var filteredSilences []monitoringv1alpha1.Silence
	{
		filteredSilences, err = r.loadSilences(ctx, tags)
		if err != nil {
			return microerror.Mask(err)
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

	for i, silence := range filteredSilences {
		if !silenceInList(silence, currentSilences.Items) {
			// create desired silences
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("creating desired silence CR %#q", silence.Name))

			err = ctrlClient.Create(ctx, &filteredSilences[i])
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("desired silence CR %#q has been created", silence.Name))
		} else {
			// update desired silences
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("updating desired silence CR %#q", silence.Name))
			existingSilence := getSilenceInList(silence, currentSilences.Items)
			updateMeta(existingSilence, &filteredSilences[i])
			err = ctrlClient.Update(ctx, &filteredSilences[i])
			if err != nil {
				return microerror.Mask(err)
			}

			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("desired silence CR %#q has been updated", silence.Name))
		}
	}

	return nil
}

func (r *runner) getCtrlClient() (ctrlClient client.Client, err error) {
	var restConfig *rest.Config
	{
		c := k8srestconfig.Config{
			Logger: r.logger,

			InCluster:  r.flag.KubernetesInCluster,
			KubeConfig: r.flag.KubernetesKubeconfig,
		}
		restConfig, err = k8srestconfig.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var k8sClients *k8sclient.Clients
	{
		c := k8sclient.ClientsConfig{
			Logger:     r.logger,
			RestConfig: restConfig,
			SchemeBuilder: k8sclient.SchemeBuilder{
				monitoringv1alpha1.AddToScheme,
			},
		}

		k8sClients, err = k8sclient.NewClients(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return k8sClients.CtrlClient(), nil
}

func updateMeta(c, d metav1.Object) {
	d.SetGenerateName(c.GetGenerateName())
	d.SetUID(c.GetUID())
	d.SetResourceVersion(c.GetResourceVersion())
	d.SetGeneration(c.GetGeneration())
	d.SetSelfLink(c.GetSelfLink())
	d.SetCreationTimestamp(c.GetCreationTimestamp())
	d.SetDeletionTimestamp(c.GetDeletionTimestamp())
	d.SetDeletionGracePeriodSeconds(c.GetDeletionGracePeriodSeconds())
	d.SetLabels(c.GetLabels())
	d.SetAnnotations(c.GetAnnotations())
	d.SetFinalizers(c.GetFinalizers())
	d.SetOwnerReferences(c.GetOwnerReferences())
	d.SetManagedFields(c.GetManagedFields())
}

func (r *runner) loadSilences(ctx context.Context, tags map[string]string) ([]monitoringv1alpha1.Silence, error) {
	// Find yaml files.
	var silenceFiles []string
	{
		for _, dir := range r.flag.Dirs {
			files, err := findYamls(dir)
			if err != nil {
				return nil, microerror.Mask(err)
			}
			silenceFiles = append(silenceFiles, files...)
		}
	}

	// Load silences CRs from yaml files.
	var filteredSilences []monitoringv1alpha1.Silence
	for _, silenceFile := range silenceFiles {
		data, err := os.ReadFile(silenceFile)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		var silence monitoringv1alpha1.Silence
		err = yaml.Unmarshal(data, &silence)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		// Check for duplicate silences.
		if silenceInList(silence, filteredSilences) {
			r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("skip duplicated silence %#q", silence.Name))
			continue
		}

		// Filter silence by target tags.
		validSilence, err := r.isValidSilence(ctx, silence, tags)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if validSilence {
			filteredSilences = append(filteredSilences, silence)
		}
	}

	return filteredSilences, nil
}

func (r *runner) isValidSilence(ctx context.Context, silence monitoringv1alpha1.Silence, tags map[string]string) (bool, error) {
	for _, envTag := range silence.Spec.TargetTags {
		matcher, err := regexp.Compile(envTag.Value)
		if err != nil {
			return false, microerror.Mask(err)
		}

		currentTag := tags[envTag.Name]
		if !matcher.MatchString(currentTag) {
			r.logger.LogCtx(ctx, "level", "debug",
				"message", fmt.Sprintf("silence %#q does not match environment by %#q key [regexp: %#q, value: %#q]",
					silence.Name, envTag.Name, envTag.Value, currentTag))
			return false, nil
		}
	}

	return true, nil
}

func findYamls(dir string) ([]string, error) {
	var result []string

	files, err := os.ReadDir(dir)
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
	return getSilenceInList(silence, silences) != nil
}

func getSilenceInList(silence monitoringv1alpha1.Silence, silences []monitoringv1alpha1.Silence) *monitoringv1alpha1.Silence {
	for _, item := range silences {
		if item.Name == silence.Name {
			return &item
		}
	}

	return nil
}

func hasKeepAnnotation(silence monitoringv1alpha1.Silence) bool {
	keep, ok := silence.ObjectMeta.Annotations[annotation.Keep]
	return ok && keep == "true"
}
