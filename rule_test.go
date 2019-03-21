package throttleplugin

import (
	"testing"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/stretchr/testify/assert"
)

func TestNewRule(t *testing.T) {
	fields := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": "5",
	}

	r := NewRule(fields, 100)

	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, r.keys)
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, r.values)
}

func TestMatch(t *testing.T) {
	event := &beat.Event{Fields: common.MapStr{}}
	event.PutValue("a", "1")
	event.PutValue("b", "2")

	t.Run("match", func(t *testing.T) {
		r := NewRule(map[string]string{"a": "1"}, 10)
		ok, key := r.Match(event)

		assert.True(t, ok)
		assert.Equal(t, "10:1:", key)
	})
}
func BenchmarkMatch(b *testing.B) {
	fields := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": "5",
	}

	r := NewRule(fields, 100)
	event := &beat.Event{Fields: common.MapStr{}}
	event.PutValue("a", "1")
	for k, v := range fields {
		event.PutValue(k, v)
	}

	for i := 0; i < b.N; i++ {
		r.Match(event)
	}
}
