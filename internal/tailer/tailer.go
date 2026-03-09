package tailer

import (
	"context"
	"fmt"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
	"regexp"
)

// Tailer interface para implementaciones de diferentes tipos de sources
type Tailer interface {
	// Start inicia el tailing en una goroutine
	Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error)

	// Stop detiene el tailing
	Stop() error

	// GetSourceName retorna el nombre de la fuente
	GetSourceName() string
}

// BaseTailer contains common functionality for all tailers
type BaseTailer struct {
	SourceName    string
	HealthMonitor *health.Monitor
}

// ReportHealthy marca la fuente como saludable
func (b *BaseTailer) ReportHealthy() {
	if b.HealthMonitor != nil {
		b.HealthMonitor.MarkHealthy(b.SourceName)
	}
}

// ReportFailed marca la fuente como fallida
func (b *BaseTailer) ReportFailed(err error) {
	if b.HealthMonitor != nil {
		b.HealthMonitor.MarkFailed(b.SourceName, err)
	}
}

// ReportDegraded marca la fuente como degradada
func (b *BaseTailer) ReportDegraded(err error) {
	if b.HealthMonitor != nil {
		b.HealthMonitor.MarkDegraded(b.SourceName, err)
	}
}

// ReportStopped marca la fuente como detenida
func (b *BaseTailer) ReportStopped() {
	if b.HealthMonitor != nil {
		b.HealthMonitor.MarkStopped(b.SourceName)
	}
}

// GetSourceName retorna el nombre de la fuente
func (b *BaseTailer) GetSourceName() string {
	return b.SourceName
}

// safeNamePattern validates names passed to external commands (container names, unit names).
// Allows alphanumeric, dash, underscore, dot, colon, and @ (for systemd units).
var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:@-]*$`)

// ValidateExternalName checks that a name is safe to pass to external commands.
func ValidateExternalName(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty", kind)
	}
	if len(name) > 256 {
		return fmt.Errorf("%s name too long (max 256 chars)", kind)
	}
	if !safeNamePattern.MatchString(name) {
		return fmt.Errorf("%s name %q contains invalid characters", kind, name)
	}
	return nil
}
