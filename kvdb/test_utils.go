package kvdb

import (
	"fmt"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/lntest/leakcheck"
)

// RunTests is a helper function to run the tests in a package with
// initialization and tear-down of a test kvdb backend.
func RunTests(m *testing.M) {
	var close func() error
	if PostgresBackend {
		var err error
		close, err = StartEmbeddedPostgres()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// os.Exit() does not respect defer statements
	code := m.Run()

	if close != nil {
		err := close()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	// Check for goroutine leaks if the experiment is enabled.
	if leakErr := leakcheck.Check(); leakErr != nil {
		fmt.Fprintf(os.Stderr, "LEAK DETECTED: %v\n", leakErr)
		code = 1
	}

	os.Exit(code)
}
