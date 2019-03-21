package throttleplugin

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type RemoteConfig struct {
	Key          string       `yaml:"key"`
	DefaultLimit int64        `yaml:"default_limit"`
	Rules        []RuleConfig `yaml:"rules"`
}

type RuleConfig struct {
	Limit     int64             `yaml:"limit"`
	Selectors map[string]string `yaml:"selectors"`
}

type RemoteLimiter struct {
	url            string
	client         *http.Client
	bucketInterval int64
	buckets        int64

	mu       sync.RWMutex
	key      string
	rules    []Rule
	limiters map[string]*BucketLimiter
}

// NewRemoteLimiter creates new remote limiter instance.
func NewRemoteLimiter(url string, bucketInterval, buckets int64) (*RemoteLimiter, error) {
	rl := &RemoteLimiter{
		url:            url,
		client:         http.DefaultClient,
		bucketInterval: bucketInterval,
		buckets:        buckets,
		limiters:       make(map[string]*BucketLimiter),
	}

	return rl, nil
}

// Allow returns TRUE if event is allowed to be processed.
func (rl *RemoteLimiter) Allow(e *beat.Event) bool {
	var ts time.Time

	if tsString, err := e.GetValue("ts"); err == nil {
		ts, _ = time.Parse(time.RFC3339, tsString.(string))
	}

	if ts.IsZero() {
		ts = time.Now()
	}

	keyValue, _ := e.GetValue(rl.key)
	kv := fmt.Sprintf("%v", keyValue)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for _, r := range rl.rules {
		if matched, key := r.Match(e); matched {
			key = kv + key
			// check if we already have limiter
			limiter, ok := rl.limiters[key]
			if !ok {
				limiter = NewBucketLimiter(rl.bucketInterval, r.Limit(), rl.buckets, ts)
				rl.limiters[key] = limiter
			}

			return limiter.Allow(ts)
		}
	}

	return true
}

// Update retrieves policies from Policy Manager.
func (rl *RemoteLimiter) Update(ctx context.Context) error {
	r, err := http.NewRequest("GET", rl.url, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	res, err := rl.client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var c RemoteConfig

	if err := yaml.Unmarshal(body, &c); err != nil {
		return errors.Wrap(err, "failed to unpack config")
	}

	rules := make([]Rule, 0, len(c.Rules)+1)

	for _, l := range c.Rules {
		rules = append(rules, NewRule(l.Selectors, l.Limit))
	}

	defaultRule := NewRule(map[string]string{}, c.DefaultLimit)
	rules = append(rules, defaultRule)

	limiterTTL := time.Duration(rl.bucketInterval*rl.buckets) * time.Second
	limiterThreshold := time.Now().Add(-limiterTTL)

	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.key = c.Key
	rl.rules = rules
	for id, l := range rl.limiters {
		if l.LastUpdate().Before(limiterThreshold) {
			delete(rl.limiters, id)
		}
	}

	return nil
}

// UpdateWithInterval runs update with some interval.
func (rl *RemoteLimiter) UpdateWithInterval(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := rl.Update(ctx); err != nil {
				logp.Err("failed to update limit policies: %v", err)
			}
		}
	}
}

func (rl *RemoteLimiter) WriteStatus(w io.Writer) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, cl := range rl.limiters {
		fmt.Fprintf(w, "#%v\n\n", key)
		if err := cl.WriteStatus(w); err != nil {
			return err
		}
		fmt.Fprintln(w, "---------")
	}

	fmt.Fprintf(w, "rules: \n\n%#v", rl.rules)

	return nil
}

func fieldsHash(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		fmt.Fprintf(&b, "%v", m[k])
		b.WriteByte(';')
	}

	return b.String()
}
