// Package leakcheck provides utilities for detecting goroutine leaks using
// Go 1.26's experimental goroutineleak profile.
//
// This package requires building with GOEXPERIMENT=goroutineleakprofile.
// When the experiment is not enabled, Check() returns nil (no-op).
package leakcheck

import (
	"bytes"
	"fmt"
	"runtime/pprof"
	"strings"
)

// Check verifies that there are no leaked goroutines by examining the
// goroutineleak profile. It returns an error if any leaked goroutines are
// detected, or nil if no leaks are found.
//
// If the goroutineleak profile is not available (i.e., the binary was not
// built with GOEXPERIMENT=goroutineleakprofile), this function returns nil.
//
// This function should be called after all tests have completed, typically
// in TestMain after m.Run() returns.
func Check() error {
	prof := pprof.Lookup("goroutineleak")
	if prof == nil {
		// Profile not available, experiment not enabled.
		return nil
	}

	var buf bytes.Buffer
	if err := prof.WriteTo(&buf, 2); err != nil {
		return fmt.Errorf("failed to write goroutineleak profile: %w", err)
	}

	output := buf.String()
	if strings.Contains(output, "(leaked)") {
		return fmt.Errorf("goroutine leak detected:\n%s", output)
	}

	return nil
}
