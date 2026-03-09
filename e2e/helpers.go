//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/api/v1alpha2"
	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

var (
	k8sClient     client.Client
	amPortForward *portForward
)

// portForward holds state for a kubectl port-forward process.
type portForward struct {
	cmd       *exec.Cmd
	localPort int
}

func (pf *portForward) stop() {
	if pf.cmd != nil && pf.cmd.Process != nil {
		_ = pf.cmd.Process.Kill()
		_ = pf.cmd.Wait()
	}
}

func newK8sClient() (client.Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("getting kubeconfig: %w", err)
	}

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("adding v1alpha1 to scheme: %w", err)
	}

	err = v1alpha2.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("adding v1alpha2 to scheme: %w", err)
	}

	c, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}

	return c, nil
}

func startPortForward(namespace, target string, remotePort int) (*portForward, error) {
	localPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("getting free port: %w", err)
	}

	cmd := exec.Command(
		"kubectl", "port-forward",
		"-n", namespace,
		target,
		fmt.Sprintf("%d:%d", localPort, remotePort),
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting port-forward: %w", err)
	}

	return &portForward{cmd: cmd, localPort: localPort}, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// getActiveSilences returns non-expired silences from the Alertmanager API.
func getActiveSilences(port int) ([]alertmanager.Silence, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v2/silences", port)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var silences []alertmanager.Silence
	if err := json.Unmarshal(body, &silences); err != nil {
		return nil, err
	}

	var active []alertmanager.Silence
	for _, s := range silences {
		if s.Status != nil && s.Status.State != alertmanager.SilenceStateExpired {
			active = append(active, s)
		}
	}
	return active, nil
}

// findSilenceByComment finds an active silence with the given comment.
func findSilenceByComment(port int, comment string) (*alertmanager.Silence, error) {
	silences, err := getActiveSilences(port)
	if err != nil {
		return nil, err
	}

	for _, s := range silences {
		if s.Comment == comment {
			return &s, nil
		}
	}
	return nil, nil
}

// createNamespace creates a namespace if it doesn't already exist.
func createNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return nil
}

// deleteNamespace deletes a namespace.
func deleteNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, ns))
}
