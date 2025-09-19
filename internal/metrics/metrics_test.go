package metrics

import (
	"net/http/httptest"
	"testing"
)

func TestHandlerBasic(t *testing.T) {
	IncRequests()
	rr := httptest.NewRecorder()
	Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != 200 {
		t.Fatalf("unexpected code: %d", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Fatalf("empty metrics body")
	}
}
