package tailer

import (
	"logtailr/internal/health"
	"testing"
)

func TestKubernetesTailer_ValidPod(t *testing.T) {
	monitor := health.NewMonitor()
	kt, err := NewKubernetesTailer("default", "my-pod", "", "", "", true, monitor)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	if kt.GetSourceName() != "k8s:default/my-pod" {
		t.Errorf("source name = %q, want %q", kt.GetSourceName(), "k8s:default/my-pod")
	}
}

func TestKubernetesTailer_ValidPodNoNamespace(t *testing.T) {
	kt, err := NewKubernetesTailer("", "my-pod", "", "", "", true, nil)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	if kt.GetSourceName() != "k8s:my-pod" {
		t.Errorf("source name = %q, want %q", kt.GetSourceName(), "k8s:my-pod")
	}
}

func TestKubernetesTailer_ValidLabelSelector(t *testing.T) {
	monitor := health.NewMonitor()
	kt, err := NewKubernetesTailer("production", "", "", "app=myapp", "", true, monitor)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	if kt.GetSourceName() != "k8s:production/selector=app=myapp" {
		t.Errorf("source name = %q, want %q", kt.GetSourceName(), "k8s:production/selector=app=myapp")
	}
}

func TestKubernetesTailer_NoPodOrSelector(t *testing.T) {
	_, err := NewKubernetesTailer("default", "", "", "", "", true, nil)
	if err == nil {
		t.Fatal("expected error when neither pod nor label_selector provided")
	}
}

func TestKubernetesTailer_BothPodAndSelector(t *testing.T) {
	_, err := NewKubernetesTailer("default", "my-pod", "", "app=myapp", "", true, nil)
	if err == nil {
		t.Fatal("expected error when both pod and label_selector provided")
	}
}

func TestKubernetesTailer_InvalidPodName(t *testing.T) {
	_, err := NewKubernetesTailer("default", "../../etc/passwd", "", "", "", true, nil)
	if err == nil {
		t.Fatal("expected error for invalid pod name")
	}
}

func TestKubernetesTailer_EmptyPodName(t *testing.T) {
	// empty pod with empty selector should fail
	_, err := NewKubernetesTailer("default", "", "", "", "", true, nil)
	if err == nil {
		t.Fatal("expected error for empty pod and empty selector")
	}
}

func TestKubernetesTailer_InvalidNamespace(t *testing.T) {
	_, err := NewKubernetesTailer("../../etc", "my-pod", "", "", "", true, nil)
	if err == nil {
		t.Fatal("expected error for invalid namespace")
	}
}

func TestKubernetesTailer_InvalidContainerName(t *testing.T) {
	_, err := NewKubernetesTailer("default", "my-pod", "../../bad", "", "", true, nil)
	if err == nil {
		t.Fatal("expected error for invalid container name")
	}
}

func TestKubernetesTailer_InvalidLabelSelector(t *testing.T) {
	_, err := NewKubernetesTailer("default", "", "", "app=my;rm -rf /", "", true, nil)
	if err == nil {
		t.Fatal("expected error for invalid label selector")
	}
}

func TestKubernetesTailer_Stop(t *testing.T) {
	monitor := health.NewMonitor()
	kt, err := NewKubernetesTailer("default", "my-pod", "", "", "", true, monitor)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}

	if err := kt.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	status, ok := monitor.GetStatus("k8s:default/my-pod")
	if !ok {
		t.Fatal("expected source to be registered")
	}
	if status.Status != health.StatusStopped {
		t.Errorf("status = %q, want %q", status.Status, health.StatusStopped)
	}
}

func TestKubernetesTailer_WithContainer(t *testing.T) {
	kt, err := NewKubernetesTailer("default", "my-pod", "sidecar", "", "", true, nil)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	if kt.container != "sidecar" {
		t.Errorf("container = %q, want %q", kt.container, "sidecar")
	}
}

func TestKubernetesTailer_BuildArgs_PodWithFollow(t *testing.T) {
	kt, err := NewKubernetesTailer("production", "api-server", "app", "", "/home/user/.kube/config", true, nil)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	args := kt.buildArgs()
	expected := []string{"logs", "--kubeconfig", "/home/user/.kube/config", "-n", "production", "-f", "-c", "app", "api-server"}
	if len(args) != len(expected) {
		t.Fatalf("args len = %d, want %d: %v", len(args), len(expected), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestKubernetesTailer_BuildArgs_LabelSelector(t *testing.T) {
	kt, err := NewKubernetesTailer("staging", "", "", "app=myapp,version=v2", "", false, nil)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	args := kt.buildArgs()
	expected := []string{"logs", "-n", "staging", "-l", "app=myapp,version=v2"}
	if len(args) != len(expected) {
		t.Fatalf("args len = %d, want %d: %v", len(args), len(expected), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestKubernetesTailer_BuildArgs_MinimalPod(t *testing.T) {
	kt, err := NewKubernetesTailer("", "my-pod", "", "", "", false, nil)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}
	args := kt.buildArgs()
	expected := []string{"logs", "my-pod"}
	if len(args) != len(expected) {
		t.Fatalf("args len = %d, want %d: %v", len(args), len(expected), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestKubernetesTailer_HealthRegistration(t *testing.T) {
	monitor := health.NewMonitor()
	_, err := NewKubernetesTailer("ns", "pod1", "", "", "", true, monitor)
	if err != nil {
		t.Fatalf("NewKubernetesTailer() error = %v", err)
	}

	status, ok := monitor.GetStatus("k8s:ns/pod1")
	if !ok {
		t.Fatal("expected source to be registered in health monitor")
	}
	if status.Status != health.StatusStarting {
		t.Errorf("initial status = %q, want %q", status.Status, health.StatusStarting)
	}
}
