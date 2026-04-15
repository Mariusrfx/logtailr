package safego

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Go launches a goroutine with panic recovery. If the goroutine panics,
// the panic is logged to stderr with a stack trace. The optional onPanic
// callback is invoked after logging, allowing the caller to restart the
// goroutine or take other action.
func Go(name string, fn func(), onPanic func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = fmt.Fprintf(os.Stderr, "PANIC in goroutine %q: %v\n%s\n", name, r, debug.Stack())
				if onPanic != nil {
					onPanic()
				}
			}
		}()
		fn()
	}()
}
