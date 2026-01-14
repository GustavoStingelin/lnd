# Go version for leak detection (use rc1 until stable release)
go_version := "go1.26rc1"

# Test tags from make/testing_flags.mk
# - dev: development build tag
# - nolog: suppress verbose logging in tests
# - lowscrypt: faster scrypt for race detection tests
# - RPC_TAGS: all RPC subsystem tags
dev_tags := "dev"
log_tags := "nolog"
rpc_tags := "autopilotrpc chainrpc invoicesrpc neutrinorpc peersrpc routerrpc signrpc verrpc walletrpc watchtowerrpc wtclientrpc"
race_tags := "lowscrypt"
test_tags := dev_tags + " " + log_tags + " " + race_tags + " " + rpc_tags

# Test timeout (matches Makefile default of 180m)
test_timeout := "180m"

# Packages to skip when running leak checks (denylist regex)
leak_deny_regex := "(^|/)(itest|lntest)(/|$)"

# Test args used by leak-check recipes
leak_test_args := "-race -timeout={{test_timeout}} -count=1 -tags=\"{{test_tags}}\" -v"

# Bootstrap Go 1.26 using official go install wrapper
bootstrap-go:
    #!/usr/bin/env bash
    set -euo pipefail

    if command -v {{go_version}} &> /dev/null; then
        exit 0
    fi
    
    echo "Installing {{go_version}} via go install..."
    go install golang.org/dl/{{go_version}}@latest
    
    echo "Downloading {{go_version}} SDK..."
    {{go_version}} download
    
    echo "{{go_version}} installed successfully!"
    {{go_version}} version

_leak-setup:
    #!/usr/bin/env bash
    set -euo pipefail

    ./scripts/gen_leak_tests.sh

    export GOEXPERIMENT=goroutineleakprofile

    rm -rf leak-reports
    mkdir -p leak-reports

_leak-cleanup:
    #!/usr/bin/env bash
    set -euo pipefail

    ./scripts/clean_leak_tests.sh

_leak-run-pkg pkg:
    #!/usr/bin/env bash
    set -euo pipefail

    pkg="{{pkg}}"

    export GOEXPERIMENT=goroutineleakprofile

    pkg_file=$(echo "$pkg" | sed 's|^\./||' | tr '/.' '__')
    tmp="leak-reports/${pkg_file}.full.tmp"
    report="leak-reports/${pkg_file}.txt"

    mkdir -p leak-reports
    rm -f "$tmp" "$report"

    # Collect full output transiently, then persist only leak info.
    # NOTE: `go test` may exit non-zero for leaks, so we always parse `$tmp`.
    CGO_ENABLED=1 {{go_version}} test -race -timeout={{test_timeout}} -count=1 -tags="{{test_tags}}" -v "$pkg" >"$tmp" 2>&1 || true

    if grep -Eq "LEAK DETECTED|goroutine leak detected|\(leaked\)" "$tmp" 2>/dev/null; then
        # Start from an explicit marker if present, otherwise fall back to the
        # first leak-related line.
        awk '
            BEGIN {p=0}
            /LEAK DETECTED/ {p=1}
            /goroutine leak detected/ {p=1}
            /\(leaked\)/ {p=1}
            p {print}
        ' "$tmp" >"$report"
    else
        echo "OK" >"$report"
    fi

    rm -f "$tmp"

    # Signal failure if this package produced a leak report.
    if ! grep -q "^OK$" "$report" 2>/dev/null; then
        exit 1
    fi

# Run tests with goroutine/channel/mutex leak detection (per-package reports)
leak-check:
    #!/usr/bin/env bash
    set -euo pipefail

    just _leak-setup
    trap "just _leak-cleanup" EXIT

    echo "Enumerating packages..."
    mapfile -t packages < <(
        {{go_version}} list ./... |
        grep -vE "{{leak_deny_regex}}"
    )

    if [ ${#packages[@]} -eq 0 ]; then
        echo "No packages found for leak check"
        exit 0
    fi

    echo "Running leak detection on ${#packages[@]} packages"
    echo "Tags: {{test_tags}}"
    echo "Reports dir: leak-reports"
    echo ""

    leak_fail=0

    for pkg in "${packages[@]}"; do
        echo "=== $pkg ==="

        if ! just _leak-run-pkg "$pkg"; then
            leak_fail=1
        fi
    done

    if [ "$leak_fail" -ne 0 ]; then
        echo ""
        echo "WARNING: Goroutine leaks detected. See leak-reports/*.txt"
        exit 1
    fi

# Run leak check on specific package(s) - faster for development
leak-check-pkg +pkgs:
    #!/usr/bin/env bash
    set -euo pipefail

    if [ -z "{{pkgs}}" ]; then
        echo "Usage: just leak-check-pkg ./pkg1 ./pkg2"
        exit 1
    fi

    just _leak-setup
    trap "just _leak-cleanup" EXIT

    echo "Running leak detection on: {{pkgs}}"
    echo "Tags: {{test_tags}}"
    echo ""

    leak_fail=0
    for pkg in {{pkgs}}; do
        echo "=== $pkg ==="

        if ! just _leak-run-pkg "$pkg"; then
            leak_fail=1
        fi
    done

    if [ "$leak_fail" -ne 0 ]; then
        echo "WARNING: Goroutine leaks detected. See leak-reports/*.txt"
        exit 1
    fi

# Run leak check in parallel (faster but less granular reports)
leak-check-fast:
    #!/usr/bin/env bash
    set -euo pipefail

    # This mode intentionally writes full output. Prefer `leak-check`.
    just _leak-setup
    trap "just _leak-cleanup" EXIT

    full_report="leak-reports/full.txt"

    echo "=== Leak Detection (Fast Mode) ===" | tee "$full_report"
    echo "Go version: $({{go_version}} version)" | tee -a "$full_report"
    echo "GOEXPERIMENT: ${GOEXPERIMENT}" | tee -a "$full_report"
    echo "Tags: {{test_tags}}" | tee -a "$full_report"
    echo "Date: $(date)" | tee -a "$full_report"
    echo "" | tee -a "$full_report"

    echo "Running all tests in parallel..."

    # Run all tests at once (faster due to parallelism)
    if CGO_ENABLED=1 {{go_version}} test {{leak_test_args}} ./... 2>&1 | tee -a "$full_report"; then
        echo ""
        echo "All tests passed."
    else
        echo ""
        echo "Some tests failed. See $full_report"
    fi

    # Check for leaks
    if grep -q "LEAK DETECTED" "$full_report" 2>/dev/null; then
        echo ""
        echo "WARNING: Leaks detected!"
        grep "LEAK DETECTED" "$full_report" -B 2
        exit 1
    fi


# Clean generated files and reports
clean:
    rm -rf leak-reports/
    ./scripts/clean_leak_tests.sh 2>/dev/null || true

# Show available commands
help:
    @echo "Leak Detection Commands:"
    @echo "  just leak-check       - Run leak detection on all packages (per-package reports)"
    @echo "  just leak-check-fast  - Run leak detection in parallel (single report, faster)"
    @echo "  just leak-check-pkg   - Run leak detection on specific package(s)"
    @echo "  just bootstrap-go     - Install Go {{go_version}}"
    @echo "  just clean            - Remove generated files and reports"
    @echo ""
    @echo "Examples:"
    @echo "  just leak-check-pkg ./routing/..."
    @echo "  just leak-check-pkg ./channeldb ./invoices"
