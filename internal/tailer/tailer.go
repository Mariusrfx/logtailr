package tailer

import (
	"context"
	"logtailr/internal/health"
	"logtailr/pkg/logline"
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
