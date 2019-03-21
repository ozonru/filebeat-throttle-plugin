package throttleplugin

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type BucketLimiter struct {
	mu             sync.Mutex
	bucketInterval int64 // bucket interval in seconds (60 = 1 min)
	limit          int64 // maximum number of events per bucket
	minBucketID    int64 // minimum bucket id
	buckets        []int64
	lastUpdate     time.Time
}

func NewBucketLimiter(bucketInterval, limit, buckets int64, now time.Time) *BucketLimiter {
	return &BucketLimiter{
		bucketInterval: bucketInterval,
		limit:          limit,
		minBucketID:    timeToBucketID(now, bucketInterval) - buckets + 1,
		buckets:        make([]int64, buckets),
	}
}

// Allow returns TRUE if event is allowed to be processed.
func (bl *BucketLimiter) Allow(t time.Time) bool {
	index := timeToBucketID(t, bl.bucketInterval)

	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.lastUpdate = time.Now()

	max := bl.minBucketID + int64(len(bl.buckets)) - 1

	if index < bl.minBucketID {
		// limiter doesn't track that bucket anymore.
		return false
	}

	if index > max {
		// event from new bucket. We need to add N new buckets
		n := index - max
		for i := 0; int64(i) < n; i++ {
			bl.buckets = append(bl.buckets, 0)
		}

		// remove old ones
		bl.buckets = bl.buckets[n:]

		// and set new min index
		bl.minBucketID += n
	}

	return bl.increment(index)
}

// LastUpdate returns last Allow method call time.
func (bl *BucketLimiter) LastUpdate() time.Time {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	return bl.lastUpdate
}

// WriteStatus writes text based status into Writer.
func (bl *BucketLimiter) WriteStatus(w io.Writer) error {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	for i, value := range bl.buckets {
		fmt.Fprintf(w, "#%s: ", bucketIDToTime(int64(i)+bl.minBucketID, bl.bucketInterval))
		progress(w, value, bl.limit, 20)
		fmt.Fprintf(w, " %d/%d\n", value, bl.limit)
	}
	return nil
}

// SetLimit updates limit value.
// Note: it's allowed only to change limit, not bucketInterval.
func (bl *BucketLimiter) SetLimit(limit int64) {
	bl.mu.Lock()
	bl.limit = limit
	bl.mu.Unlock()
}

// increment adds 1 to specified bucket.
// Note: this func is not thread safe, so it must be guarded with lock.
func (сl *BucketLimiter) increment(index int64) bool {
	i := index - сl.minBucketID
	if сl.buckets[i] >= int64(сl.limit) {
		return false
	}
	сl.buckets[i]++

	return true
}

func progress(w io.Writer, current, limit, max int64) {
	p := float64(current) / float64(limit) * float64(max)

	fmt.Fprint(w, "[")
	for i := int64(0); i < max; i++ {
		if i < int64(p) {
			fmt.Fprint(w, "#")
		} else {
			fmt.Fprint(w, "_")
		}
	}
	fmt.Fprint(w, "]")
}

// bucketbucketIDToTime converts bucketID to time. This time is start of the bucket.
func bucketIDToTime(id int64, interval int64) time.Time {
	return time.Unix(id*interval, 0)
}

// timeToBucketID converts time to bucketID.
func timeToBucketID(t time.Time, interval int64) int64 {
	return t.Unix() / interval
}
