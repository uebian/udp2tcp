package service

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uebian/udp2tcp/config"
)

type udp2tcpCollector struct {
	wsReadCoroutineGauge  prometheus.Gauge
	wsWriteCoroutineGauge prometheus.Gauge
}

func Newudp2tcpCollector() *udp2tcpCollector {
	return &udp2tcpCollector{
		wsReadCoroutineGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "current_ws_read_coroutines",
				Help: "The number of goroutines that are currently reading from the WebSocket",
			},
		),
		wsWriteCoroutineGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "current_ws_write_coroutines",
				Help: "The number of goroutines that are currently writing to the WebSocket",
			},
		),
	}
}

func (c *udp2tcpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.wsReadCoroutineGauge.Desc()
	ch <- c.wsWriteCoroutineGauge.Desc()
}

func (c *udp2tcpCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- c.wsReadCoroutineGauge
	ch <- c.wsWriteCoroutineGauge
}

func ListenAndServeMetrics(cfg *config.Config) {
	http.Handle("/metrics", promhttp.Handler())

	err := http.ListenAndServe(cfg.MetricListenAddr, nil)
	if err != nil {
		log.Fatalf("Failed to start Websocket Listener: %v", err)
	}
}
