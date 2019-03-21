package throttleplugin

import (
	"bytes"
	"testing"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/stretchr/testify/assert"
)

func TestAllow(t *testing.T) {
	now := time.Date(2018, 12, 19, 19, 30, 25, 0, time.UTC)

	t.Run("current bucket overflow", func(t *testing.T) {
		cl, err := NewConditionLimiter(map[string]string{}, 60, 2, 5, now)
		if err != nil {
			t.Fatal(err)
		}

		assert.True(t, cl.Allow(now))
		assert.True(t, cl.Allow(now))
		assert.False(t, cl.Allow(now), "bucket must be exceeded")
	})

	t.Run("update limit", func(t *testing.T) {
		cl, err := NewConditionLimiter(map[string]string{}, 60, 2, 5, now)
		if err != nil {
			t.Fatal(err)
		}

		assert.True(t, cl.Allow(now))
		assert.True(t, cl.Allow(now))
		assert.False(t, cl.Allow(now))

		cl.SetLimit(3)
		assert.True(t, cl.Allow(now))
		assert.False(t, cl.Allow(now), "bucket must be exceeded")
	})

	t.Run("add to oldest bucket", func(t *testing.T) {
		cl, err := NewConditionLimiter(map[string]string{}, 60, 2, 5, now)
		if err != nil {
			t.Fatal(err)
		}

		ts := now.Add(-4 * time.Minute)

		assert.True(t, cl.Allow(ts))
		assert.True(t, cl.Allow(ts))
	})

	t.Run("old bucket shifts", func(t *testing.T) {
		cl, err := NewConditionLimiter(map[string]string{}, 60, 2, 5, now)
		if err != nil {
			t.Fatal(err)
		}

		oldest := now.Add(-4 * time.Minute)

		assert.True(t, cl.Allow(now.Add(time.Minute)))
		assert.False(t, cl.Allow(oldest), "oldest bucket must be shifted")
	})
}

func TestCheck(t *testing.T) {
	cl, _ := NewConditionLimiter(map[string]string{"a": "1"}, 60, 2, 5, time.Now())

	t.Run("valid", func(t *testing.T) {
		event := &beat.Event{Fields: common.MapStr{}}
		event.PutValue("a", "1")

		assert.True(t, cl.Check(event))
	})

	t.Run("invalid", func(t *testing.T) {
		event := &beat.Event{Fields: common.MapStr{}}
		event.PutValue("a", "2")

		assert.False(t, cl.Check(event))
	})
}

func BenchmarkAllow(b *testing.B) {
	b.Run("never overflows", func(b *testing.B) {
		t := time.Now()
		cl, _ := NewConditionLimiter(
			map[string]string{},
			1,
			int64(b.N),
			10,
			t,
		)

		for i := 0; i < b.N; i++ {
			cl.Allow(t)
		}
	})

	b.Run("always overflows", func(b *testing.B) {
		t := time.Now()
		cl, _ := NewConditionLimiter(
			map[string]string{},
			1,
			0,
			10,
			t,
		)

		for i := 0; i < b.N; i++ {
			cl.Allow(t)
		}
	})

	b.Run("always shifts", func(b *testing.B) {
		t := time.Now()
		cl, _ := NewConditionLimiter(
			map[string]string{},
			1,
			0,
			10,
			t,
		)

		for i := 0; i < b.N; i++ {
			cl.Allow(t.Add(time.Duration(i) * time.Second))
		}
	})
}

func TestConditionLimiter_WriteStatus(t *testing.T) {
	current := time.Local
	time.Local = time.UTC
	defer func() {
		time.Local = current
	}()
	tt := time.Date(2018, 12, 19, 19, 30, 25, 0, time.UTC)
	cl, _ := NewConditionLimiter(map[string]string{"a": "1"}, 60, 2, 2, tt)
	cl.Allow(tt)

	var b bytes.Buffer
	cl.WriteStatus(&b)

	assert.Equal(t, `map[a:1]
#2018-12-19 19:29:00 +0000 UTC: [____________________] 0/2
#2018-12-19 19:30:00 +0000 UTC: [##########__________] 1/2
`, b.String())
}

func TestProgress(t *testing.T) {
	var b bytes.Buffer

	progress(&b, 3, 5, 10)
	assert.Equal(t, "[######____]", b.String())
}

func BenchmarkTimeToMinute(b *testing.B) {
	f := time.Now()
	for i := 0; i < b.N; i++ {
		timeToBucketID(f, 1)
	}
}

func BenchmarkIDToTime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bucketIDToTime(1, 1)
	}
}
