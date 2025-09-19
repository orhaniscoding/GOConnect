package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

var reqTotal uint64

// IncRequests increments the request counter.
func IncRequests() { atomic.AddUint64(&reqTotal, 1) }

// Handler exposes a simple Prometheus-like text output for a couple of counters.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP goconnect_requests_total Total HTTP requests handled.\n")
		fmt.Fprintf(w, "# TYPE goconnect_requests_total counter\n")
		fmt.Fprintf(w, "goconnect_requests_total %d\n", atomic.LoadUint64(&reqTotal))
	})
}
