package throttleplugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testServer(t *testing.T, body []byte) (url string, closeFn func()) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}
	s := httptest.NewServer(http.HandlerFunc(h))

	return s.URL, func() { s.Close() }
}

func TestRemoteLimiter_Update(t *testing.T) {
	response := `key: id
default_limit: 1
rules:
  - limit: 100
    selectors:
      id: foo`
	url, closeFn := testServer(t, []byte(response))
	defer closeFn()

	l, _ := NewRemoteLimiter(url, 1, 10)
	assert.NoError(t, l.Update(context.Background()))
}
