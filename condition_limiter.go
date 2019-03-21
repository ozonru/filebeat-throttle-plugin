package throttleplugin

import (
	"fmt"
	"io"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/conditions"
	"github.com/pkg/errors"
)

// ConditionLimiter first checks if event is valid for specified conditions and then applies rate limiting.
type ConditionLimiter struct {
	condition conditions.Condition
	keys      []string          // sorted list of used keys is used for combining limiter key.
	fields    map[string]string // used only for WriteStatus functionality, because it's hard to pretty print conditions.

	bl *BucketLimiter
}

// NewConditionLimiter returns new ConditionLimiter instance.
func NewConditionLimiter(fields map[string]string, bucketInterval, limit, buckets int64, now time.Time) (*ConditionLimiter, error) {
	f := conditions.Fields{}
	if err := f.Unpack(prepareFields(fields)); err != nil {
		return nil, errors.Wrap(err, "failed to unpack fields")
	}

	cs, err := conditions.NewCondition(&conditions.Config{Equals: &f})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create conditions")
	}

	return &ConditionLimiter{
		fields:    fields,
		condition: cs,
		bl:        NewBucketLimiter(bucketInterval, limit, buckets, now),
	}, nil
}

// WriteStatus writes text based status into Writer.
func (cl *ConditionLimiter) WriteStatus(w io.Writer) error {
	fmt.Fprintf(w, "%v\n", cl.fields)

	return cl.bl.WriteStatus(w)
}

// SetLimit updates limit value.
// Note: it's allowed only to change limit, not bucketInterval.
func (cl *ConditionLimiter) SetLimit(limit int64) {
	cl.bl.SetLimit(limit)
}

// Check checks if event satisfies condition.
func (cl *ConditionLimiter) Check(e *beat.Event) bool {
	return cl.condition.Check(e)
}

// Allow returns TRUE if event is allowed to be processed.
func (cl *ConditionLimiter) Allow(t time.Time) bool {
	return cl.bl.Allow(t)
}

func prepareFields(m map[string]string) map[string]interface{} {
	r := make(map[string]interface{}, len(m))
	for k, v := range m {
		r[k] = v
	}

	return r
}
