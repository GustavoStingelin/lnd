# Fix all goroutine leaks

I'm proposing we add goroutine leak detection to our test suite using Go 1.26 `GOEXPERIMENT=goroutineleakprofile` **and** then systematically fix the leaks it finds.

Initial runs show several packages have leaks, and this issue is tracking the follow-up work to drive those reports to `OK`.

## Reference implementation (diff)

Work-in-progress implementation that generates `leak-reports/` and adds leak-check tooling lives on this branch comparison (not a PR yet):

https://github.com/lightningnetwork/lnd/compare/master...GustavoStingelin:lnd:bug/goleak?expand=1

## Example leaks

**`chainio` package:**
- `BlockbeatDispatcher.dispatchBlocks` goroutine still running
- Likely: test didn't call `Stop()` or forgot to cancel context

**`bitcoindnotify` package:**
- Multiple goroutines from btcd/btcwallet test harness
- Likely: didn't properly shut down the test harness

**See `leak-reports/` for the full list**

## How to fix a leak

### 1. Find the leak

Pick a package from `leak-reports/` and run:

Note: the leak-check tooling is defined by the WIP diff above.

`just leak-check-pkg ./<pkg>`

Look at the stack trace to see where the goroutine is stuck.

### 2. Understand what kind of leak it is

**Test-only leak:** Test code didn't clean up (missing `Stop()`, `Close()`, `cancel()`, etc.)

**Production leak:** Real code starts a goroutine but has no way to stop it properly.

### 3. Fix it

**In tests:**
- Use `t.Cleanup(...)` to ensure cleanup happens
- Use `t.Context()` to automatically cancel goroutines when test ends
- Call `Stop()` or `Close()` on any background services

**In production code:**
- Use `context.Context` for cancellation (not custom quit channels)
- Make sure goroutines can be stopped via `Stop()`, `Close()`, or context cancellation
- Use `WaitGroup` or `errgroup` so you know when goroutines actually finish

**External dependencies:**
- If the leak is in test harness code (btcd, btcwallet), make sure you're calling their shutdown methods correctly.
- If teardown is correct but the dependency still leaks under `GOEXPERIMENT=goroutineleakprofile`, please document the investigation and (if appropriate) open an upstream issue. The goal here is still to drive `leak-reports/` to `OK` in our CI/repo workflow.

### 4. Verify the fix

`just leak-check-pkg ./<pkg>`

Should show `OK`.

## Common patterns

### Blocked channel operations
**Symptom:** `chan receive (leaked)` or `chan send (leaked)`
**Cause:** Goroutine waiting on a channel that never sends/closes or has no receiver
**Impact:** In production, this means memory leaks and goroutines piling up indefinitely
**Fix:** Ensure channels are closed, add context cancellation, or use buffered channels with timeout

### Select without exit
**Symptom:** `select (leaked)`
**Cause:** Goroutine stuck in select with no cancellation path
**Impact:** Background workers that never stop, consuming resources forever
**Fix:** Always include `case <-ctx.Done():` in select statements

### Forgotten timers/tickers
**Symptom:** `time.Sleep` or `ticker.C` in stack trace
**Cause:** Timer or ticker still running after component stopped
**Impact:** Goroutines accumulate over time, eventual resource exhaustion
**Fix:** Call `timer.Stop()` or `ticker.Stop()` in cleanup, use `defer`

### Missing synchronization
**Symptom:** Goroutine still running after `Stop()` or `Close()` called
**Cause:** No `WaitGroup` or `errgroup` to ensure goroutine actually exits
**Impact:** Shutdown appears complete but work continues, causing corruption or panics
**Fix:** Use `sync.WaitGroup` or `errgroup.Group` to track goroutine lifecycle

### Context not propagated
**Symptom:** Long-running operation with no cancellation
**Cause:** Started goroutine without passing context through
**Impact:** Goroutines can't be canceled, making graceful shutdown impossible
**Fix:** Accept `context.Context` as first parameter, check `ctx.Done()` regularly

### Test harness not cleaned up
**Symptom:** RPC handlers, wallet workers, or external service goroutines
**Cause:** Test infrastructure (btcd, btcwallet, etc.) not properly shut down
**Impact:** Test isolation broken, subsequent tests may fail or behave incorrectly
**Fix:** Call all shutdown/cleanup methods, use `t.Cleanup()` to ensure execution

## Reference

Goroutine Leak patterns: https://alexrios.me/blog/goroutine-leak-detection-patterns/

Implementation diff (WIP): https://github.com/lightningnetwork/lnd/compare/master...GustavoStingelin:lnd:bug/goleak?expand=1

Go 1.26 release notes: https://go.dev/doc/go1.26#goroutineleak-profiles
