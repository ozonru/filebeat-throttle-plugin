package throttleplugin

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/processors"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var unknownValue = "UNKNOWN"

// Config defines processor configuration.
type Config struct {
	PolicyHost           string        `config:"policy_host"`
	PolicyUpdateInterval time.Duration `config:"policy_update_interval"`
	PrometheusPort       int           `config:"prometheus_port"`

	BucketSize int64 `config:"bucket_size"`
	Buckets    int64 `config:"buckets"`

	MetricName   string         `config:"metric_name"`
	MetricLabels []LabelMapping `config:"metric_labels"`
}

type LabelMapping struct {
	From string `config:"from"`
	To   string `config:"to"`
}

func (c Config) GetMetricLabels() []string {
	labels := make([]string, len(c.MetricLabels))
	for i, lm := range c.MetricLabels {
		labels[i] = lm.To
	}

	return labels
}

func (c Config) GetFields() []string {
	fields := make([]string, len(c.MetricLabels))
	for i, lm := range c.MetricLabels {
		fields[i] = lm.From
	}

	return fields
}

// compile-time check that Processor implements processors.Processor interface.
var _ processors.Processor = &Processor{}

// Processor make rate-limiting for messages.
type Processor struct {
	metric     *prometheus.CounterVec
	fields     []string  // list of fields that will be used as labels in metrics
	valuesPool sync.Pool // used for zero allocation metric observations

	limiter   *RemoteLimiter
	throttled int64

	httpServer *http.Server
}

// NewProcessor returns new processor instance.
func NewProcessor(cfg *common.Config) (processors.Processor, error) {
	return newProcessor(cfg)
}

func newProcessor(cfg *common.Config) (*Processor, error) {
	var c Config
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("failed to unpack config: %v", err)
	}

	vec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "filebeat",
			Name:      c.MetricName,
			Help:      c.MetricName,
		},
		append(c.GetMetricLabels(), "throttled"),
	)

	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()

		for range t.C {
			vec.Reset()
		}
	}()
	prometheus.MustRegister(vec)

	limiter, err := NewRemoteLimiter(c.PolicyHost, c.BucketSize, c.Buckets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RemoteLimiter")
	}

	processor := &Processor{
		metric: vec,
		fields: c.GetFields(),
		valuesPool: sync.Pool{
			New: func() interface{} {
				// "+1" is used because last label is always "throttled".
				size := len(c.GetFields()) + 1
				return make([]string, size)
			},
		},
		limiter: limiter,
	}

	logp.Info("listening prometheus handler on port: %v", c.PrometheusPort)
	processor.RunHTTPHandlers(c.PrometheusPort)

	logp.Info("initial update for policies...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := limiter.Update(ctx); err != nil {
		logp.Err("failed to make initial policy update: %v. Using default", err)
	}

	logp.Info("limit policy url: %v, updateInterval: %v", c.PolicyHost, c.PolicyUpdateInterval)
	go limiter.UpdateWithInterval(context.Background(), c.PolicyUpdateInterval)

	return processor, nil
}

// RunHTTPHandlers runs prometheus handler on specified port.
func (mp *Processor) RunHTTPHandlers(port int) {
	h := promhttp.Handler()
	mux := http.NewServeMux()
	mux.Handle("/metrics", h)
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		mp.limiter.WriteStatus(w)
	})
	logp.Info("starting prometheus handler on :%v", port)
	srv := &http.Server{Addr: fmt.Sprintf(":%v", port), Handler: mux}
	mp.httpServer = srv

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logp.Err("failed to run prometheus handler: %v", err)
		}
	}()
}

func (mp *Processor) Close() error {
	return mp.httpServer.Close()
}

func (mp *Processor) Run(event *beat.Event) (*beat.Event, error) {
	v := mp.valuesPool.Get()
	values := v.([]string)
	defer func() {
		// no reslicing is required before returning to pool because slice size is always the same.
		mp.valuesPool.Put(v)
	}()

	for i, f := range mp.fields {
		v, err := event.GetValue(f)
		if err == common.ErrKeyNotFound {
			values[i] = unknownValue
			continue
		}

		switch t := v.(type) {
		case string:
			values[i] = t
		default:
			values[i] = fmt.Sprintf("%v", v)
		}
	}

	if !mp.limiter.Allow(event) {
		mp.throttled++
		values[len(values)-1] = "y"
		mp.metric.WithLabelValues(values...).Inc()
		return nil, nil
	}

	mp.throttled = 0
	values[len(values)-1] = "n"
	mp.metric.WithLabelValues(values...).Inc()

	return event, nil
}

func (mp *Processor) update(ctx context.Context) error {
	return nil
}

func (mp *Processor) String() string {
	return "metrics_processor"
}
