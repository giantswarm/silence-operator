package sync

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/giantswarm/k8sclient/v6/pkg/k8sclient"
	"github.com/giantswarm/k8sclient/v6/pkg/k8srestconfig"
	"github.com/giantswarm/k8smetadata/pkg/annotation"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	// Load desired silences from files.
	var filteredSilences []monitoringv1alpha1.Silence
	{
		labelSelector, err := r.loadTags()
		if err != nil {
			return microerror.Mask(err)
		}

		filteredSilences, err = r.loadSilences(ctx, labelSelector)
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

func (r *runner) loadTags() (labels.Set, error) {
	selector := strings.Join(r.flag.Tags, ",")

	labelsMap, err := labels.ConvertSelectorToLabelsMap(selector)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return labelsMap, nil
}

func (r *runner) loadSilences(ctx context.Context, labelSelector labels.Set) ([]monitoringv1alpha1.Silence, error) {
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
		data, err := ioutil.ReadFile(silenceFile)
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

		// Filter silence by labels.
		valid, err := r.isValidSilence(ctx, silence, labelSelector)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		if valid {
			filteredSilences = append(filteredSilences, silence)
		}
	}

	return filteredSilences, nil
}

func (r *runner) isValidSilence(ctx context.Context, silence monitoringv1alpha1.Silence, labelsMap labels.Set) (bool, error) {

	silenceLabels := silence.Spec.TargetTags
	if silenceLabels == nil {
		// This is required otherwise a nil value lead to matching nothing, while an empty value matches everyting.
		silenceLabels = &metav1.LabelSelector{}
	}

	selector, err := metav1.LabelSelectorAsSelector(silenceLabels)
	if err != nil {
		return false, microerror.Mask(err)
	}

	valid := selector.Matches(labelsMap)

	if !valid {
		r.logger.LogCtx(ctx, "level", "debug",
			"message", fmt.Sprintf("silence %#q with labels %#q does not match %#q selector",
				silence.Name, selector.String(), labelsMap.String()))
		return false, nil
	}

	return true, nil
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
