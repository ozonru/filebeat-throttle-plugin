package throttleplugin

import (
	"bytes"
	"sort"
	"strconv"
	"sync"
	"unsafe"

	"github.com/elastic/beats/libbeat/beat"
)

var sbPool sync.Pool

func init() {
	sbPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
}

type Rule struct {
	keys   []string // sorted list of used keys is used for combining limiter key.
	values []string // values to check against. order is the same as for keys.
	limit  int64

	// baseKey contains strings representation of limit to increase Match performance.
	// strconv.Itoa makes 2 allocations with 32 bytes for each call.
	baseKey string
}

// NewRule returns new Rule instance.
func NewRule(fields map[string]string, limit int64) Rule {
	var (
		keys   = make([]string, 0, len(fields))
		values = make([]string, len(fields))
	)

	for k := range fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {
		values[i] = fields[k]
	}

	return Rule{
		keys:    keys,
		values:  values,
		limit:   limit,
		baseKey: strconv.FormatInt(limit, 10),
	}
}

// Limit returns current limit.
func (r Rule) Limit() int64 {
	return r.limit
}

// Match checks if event has the same field values as expected.
func (r Rule) Match(e *beat.Event) (ok bool, key string) {
	b := sbPool.Get()
	sb := b.(*bytes.Buffer)
	sb.Reset()
	defer func() {
		sbPool.Put(b)
	}()
	sb.WriteString(r.baseKey)
	sb.WriteByte(':')
	for i, k := range r.keys {
		v, err := e.GetValue(k)
		if err != nil {
			return false, ""
		}

		sv, ok := v.(string)
		if !ok {
			// only strings values are supported
			return false, ""
		}

		if sv != r.values[i] {
			return false, ""
		}

		sb.WriteString(sv)
		sb.WriteByte(':')
	}

	buf := sb.Bytes()
	// zero-allocation convertion from []bytes to string.
	s := *(*string)(unsafe.Pointer(&buf))

	return true, s
}
