package proxy

import "sync/atomic"

var activeRequests atomic.Int64

func IncActive() { activeRequests.Add(1) }
func DecActive() { activeRequests.Add(-1) }
func ActiveCount() int64 { return activeRequests.Load() }
