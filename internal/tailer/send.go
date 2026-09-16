package tailer

import "context"

// sendErr sends err to errChan without blocking when the context is
// cancelled (e.g. during shutdown, when the pipeline stops consuming errors).
func sendErr(ctx context.Context, errChan chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errChan <- err:
	case <-ctx.Done():
	}
}
