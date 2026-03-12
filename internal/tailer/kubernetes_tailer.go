package tailer

import (
	"bufio"
	"context"
	"fmt"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// safeLabelSelectorPattern validates label selectors before passing to kubectl.
var safeLabelSelectorPattern = regexp.MustCompile(`^[a-zA-Z0-9_./-]+(=[a-zA-Z0-9_./-]+|!=[a-zA-Z0-9_./-]+| in \([a-zA-Z0-9_.,/ -]+\)| notin \([a-zA-Z0-9_.,/ -]+\))?(,[a-zA-Z0-9_./-]+(=[a-zA-Z0-9_./-]+|!=[a-zA-Z0-9_./-]+| in \([a-zA-Z0-9_.,/ -]+\)| notin \([a-zA-Z0-9_.,/ -]+\))?)*$`)

// KubernetesTailer reads log lines from Kubernetes pods using `kubectl logs`.
type KubernetesTailer struct {
	BaseTailer
	namespace     string
	pod           string
	container     string
	labelSelector string
	kubeconfig    string
	follow        bool
	cancel        context.CancelFunc
}

// NewKubernetesTailer creates a new KubernetesTailer.
// Either pod or labelSelector must be provided, but not both.
func NewKubernetesTailer(namespace, pod, container, labelSelector, kubeconfig string, follow bool, healthMonitor *health.Monitor) (*KubernetesTailer, error) {
	if pod == "" && labelSelector == "" {
		return nil, fmt.Errorf("kubernetes tailer requires a pod name or label selector")
	}
	if pod != "" && labelSelector != "" {
		return nil, fmt.Errorf("kubernetes tailer cannot have both pod and label_selector")
	}

	if pod != "" {
		if err := ValidateExternalName(pod, "pod"); err != nil {
			return nil, err
		}
	}
	if container != "" {
		if err := ValidateExternalName(container, "container"); err != nil {
			return nil, err
		}
	}
	if namespace != "" {
		if err := ValidateExternalName(namespace, "namespace"); err != nil {
			return nil, err
		}
	}
	if labelSelector != "" {
		if len(labelSelector) > 1024 {
			return nil, fmt.Errorf("label selector too long (max 1024 chars)")
		}
		if !safeLabelSelectorPattern.MatchString(labelSelector) {
			return nil, fmt.Errorf("label selector %q contains invalid characters", labelSelector)
		}
	}

	// Build source name
	var name string
	if pod != "" {
		name = "k8s:" + pod
	} else {
		name = "k8s:selector=" + labelSelector
	}
	if namespace != "" {
		name = "k8s:" + namespace + "/" + strings.TrimPrefix(name, "k8s:")
	}

	kt := &KubernetesTailer{
		BaseTailer: BaseTailer{
			SourceName:    name,
			HealthMonitor: healthMonitor,
		},
		namespace:     namespace,
		pod:           pod,
		container:     container,
		labelSelector: labelSelector,
		kubeconfig:    kubeconfig,
		follow:        follow,
	}

	if healthMonitor != nil {
		healthMonitor.RegisterSource(name)
	}

	return kt, nil
}

// Start begins reading pod logs.
func (kt *KubernetesTailer) Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	ctx, kt.cancel = context.WithCancel(ctx)

	go kt.runWithReconnect(ctx, out, errChan)
}

// Stop signals the tailer to stop.
func (kt *KubernetesTailer) Stop() error {
	if kt.cancel != nil {
		kt.cancel()
	}
	kt.ReportStopped()
	return nil
}

func (kt *KubernetesTailer) runWithReconnect(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) {
	delay := reconnectBaseDelay

	for {
		exited := kt.run(ctx, out, errChan)

		if ctx.Err() != nil {
			return
		}

		if !exited {
			return
		}

		// Pod exited — attempt reconnect with backoff
		kt.ReportDegraded(fmt.Errorf("pod log stream ended, reconnecting in %s", delay))
		errChan <- fmt.Errorf("k8s: pod log stream ended, reconnecting in %s", delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay = delay * 2
		if delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

// run executes a single kubectl logs session. Returns true if the process started
// and then exited (eligible for reconnect), false if it failed to start.
func (kt *KubernetesTailer) run(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error) bool {
	args := kt.buildArgs()

	cmd := exec.CommandContext(ctx, "kubectl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		kt.ReportFailed(err)
		errChan <- fmt.Errorf("kubectl stdout pipe: %w", err)
		return false
	}

	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		kt.ReportFailed(err)
		errChan <- fmt.Errorf("kubectl logs failed: %w", err)
		return false
	}

	kt.ReportHealthy()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, readBufferSize), maxLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return true
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		ll := &logline.LogLine{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   line,
			Source:    kt.SourceName,
			Fields:    make(map[string]interface{}),
		}

		select {
		case out <- ll:
		case <-ctx.Done():
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		default:
			kt.ReportDegraded(err)
			errChan <- fmt.Errorf("kubectl logs read error: %w", err)
		}
	}

	if err := cmd.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
			// Process exited — eligible for reconnect
		}
	}

	return true
}

// buildArgs constructs the kubectl logs arguments.
func (kt *KubernetesTailer) buildArgs() []string {
	args := []string{"logs"}

	if kt.kubeconfig != "" {
		args = append(args, "--kubeconfig", kt.kubeconfig)
	}
	if kt.namespace != "" {
		args = append(args, "-n", kt.namespace)
	}
	if kt.follow {
		args = append(args, "-f")
	}
	if kt.container != "" {
		args = append(args, "-c", kt.container)
	}

	if kt.labelSelector != "" {
		args = append(args, "-l", kt.labelSelector)
	} else {
		args = append(args, kt.pod)
	}

	return args
}
