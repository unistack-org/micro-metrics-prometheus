package prometheus

import (
	"context"
	"fmt"

	"github.com/micro/go-micro/server"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	defaultMetricPrefix = "micro"
)

func NewHandlerWrapper(opts ...server.Option) server.HandlerWrapper {
	md := make(map[string]string)
	sopts := server.Options{}

	for _, opt := range opts {
		opt(&sopts)
	}

	for k, v := range sopts.Metadata {
		md[fmt.Sprintf("%s_%s", defaultMetricPrefix, k)] = v
	}
	if len(sopts.Name) > 0 {
		md[fmt.Sprintf("%s_%s", defaultMetricPrefix, "name")] = sopts.Name
	}
	if len(sopts.Id) > 0 {
		md[fmt.Sprintf("%s_%s", defaultMetricPrefix, "id")] = sopts.Id
	}
	if len(sopts.Version) > 0 {
		md[fmt.Sprintf("%s_%s", defaultMetricPrefix, "version")] = sopts.Version
	}

	opsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        fmt.Sprintf("%s_request_total", defaultMetricPrefix),
			Help:        "How many go-micro requests processed, partitioned by method and status",
			ConstLabels: md,
		},
		[]string{"method", "status"},
	)

	timeCounterSummary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:        fmt.Sprintf("%s_upstream_latency_microseconds", defaultMetricPrefix),
			Help:        "Service backend method request latencies in microseconds",
			ConstLabels: md,
		},
		[]string{"method"},
	)

	timeCounterHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        fmt.Sprintf("%s_request_duration_seconds", defaultMetricPrefix),
			Help:        "Service method request time in seconds",
			ConstLabels: md,
		},
		[]string{"method"},
	)

	prometheus.MustRegister(opsCounter)
	prometheus.MustRegister(timeCounterSummary)
	prometheus.MustRegister(timeCounterHistogram)

	return func(fn server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			name := req.Endpoint()

			timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
				us := v * 1000000 // make microseconds
				timeCounterSummary.WithLabelValues(name).Observe(us)
				timeCounterHistogram.WithLabelValues(name).Observe(v)
			}))
			defer timer.ObserveDuration()

			err := fn(ctx, req, rsp)
			if err == nil {
				opsCounter.WithLabelValues(name, "success").Inc()
			} else {
				opsCounter.WithLabelValues(name, "fail").Inc()
			}

			return err
		}
	}
}
