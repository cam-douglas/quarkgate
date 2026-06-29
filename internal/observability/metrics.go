package observability

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Simple Prometheus-compatible counters for MVP (no client_golang dependency).

var (
	requestsTotal          atomic.Int64
	creditsCapturedMicro   atomic.Int64
	holdFailuresTotal      atomic.Int64
	meterEventsProcessed   atomic.Int64
	meterDLQTotal          atomic.Int64
	balanceDriftTotal      atomic.Int64
)

func IncRequests() { requestsTotal.Add(1) }
func IncCreditsCaptured(micro int64) { creditsCapturedMicro.Add(micro) }
func IncHoldFailures() { holdFailuresTotal.Add(1) }
func IncMeterProcessed() { meterEventsProcessed.Add(1) }
func IncMeterDLQ() { meterDLQTotal.Add(1) }
func IncBalanceDrift() { balanceDriftTotal.Add(1) }

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP quarkgate_requests_total Total HTTP requests handled\n")
		fmt.Fprintf(w, "# TYPE quarkgate_requests_total counter\n")
		fmt.Fprintf(w, "quarkgate_requests_total %d\n", requestsTotal.Load())
		fmt.Fprintf(w, "# HELP quarkgate_credits_captured_micro Total credits captured in micro_credits\n")
		fmt.Fprintf(w, "# TYPE quarkgate_credits_captured_micro counter\n")
		fmt.Fprintf(w, "quarkgate_credits_captured_micro %d\n", creditsCapturedMicro.Load())
		fmt.Fprintf(w, "# HELP quarkgate_hold_failures_total Solvency hold failures\n")
		fmt.Fprintf(w, "# TYPE quarkgate_hold_failures_total counter\n")
		fmt.Fprintf(w, "quarkgate_hold_failures_total %d\n", holdFailuresTotal.Load())
		fmt.Fprintf(w, "# HELP quarkgate_meter_events_processed_total Meter events processed by worker\n")
		fmt.Fprintf(w, "# TYPE quarkgate_meter_events_processed_total counter\n")
		fmt.Fprintf(w, "quarkgate_meter_events_processed_total %d\n", meterEventsProcessed.Load())
		fmt.Fprintf(w, "# HELP quarkgate_meter_dlq_total Meter events sent to DLQ\n")
		fmt.Fprintf(w, "# TYPE quarkgate_meter_dlq_total counter\n")
		fmt.Fprintf(w, "quarkgate_meter_dlq_total %d\n", meterDLQTotal.Load())
		fmt.Fprintf(w, "# HELP quarkgate_balance_drift_total Balance reconciliation drifts detected\n")
		fmt.Fprintf(w, "# TYPE quarkgate_balance_drift_total counter\n")
		fmt.Fprintf(w, "quarkgate_balance_drift_total %d\n", balanceDriftTotal.Load())
	})
}
