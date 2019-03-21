package throttleplugin

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/stretchr/testify/assert"
)

var config = `---
metric_name: %metric%
policy_update_interval: 60m
labels: 
  - 
    from: input.rrrr
    to: input
  - 
    from: host.name
    to: host`

func getConfig() []byte {
	name := fmt.Sprintf("metric_%v", rand.Int63())
	cfg := strings.Replace(config, "%metric%", name, -1)

	return []byte(cfg)
}

func TestConfig_GetMetricLabels(t *testing.T) {
	c := Config{
		MetricLabels: []LabelMapping{
			{"from_1", "to_1"},
			{"from_2", "to_2"},
			{"from_3", "to_3"},
		},
	}

	assert.Equal(t, []string{"to_1", "to_2", "to_3"}, c.GetMetricLabels())
}

func TestConfig_GetFields(t *testing.T) {
	c := Config{
		MetricLabels: []LabelMapping{
			{"from_1", "to_1"},
			{"from_2", "to_2"},
			{"from_3", "to_3"},
		},
	}

	assert.Equal(t, []string{"from_1", "from_2", "from_3"}, c.GetFields())
}

func BenchmarkRun(b *testing.B) {
	cfg, err := common.NewConfigWithYAML(getConfig(), "test")
	if err != nil {
		b.Fatal(err)
	}
	mp, err := newProcessor(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer mp.Close()

	event := &beat.Event{
		Fields: common.MapStr{},
	}
	event.PutValue("input.type", "foo")
	event.PutValue("host.name", "bar")
	for i := 0; i < b.N; i++ {
		mp.Run(event)
	}
}
