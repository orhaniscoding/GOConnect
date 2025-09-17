package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"goconnect/internal/config"
	"goconnect/internal/core"
)

func testAPI(t *testing.T) *API {
	cfg := config.Default("en")
	cfg.Networks = []config.Network{{ID: "n1", Name: "Net1", Joined: true}}
	st := core.NewState(core.Settings{Port: cfg.Port, MTU: cfg.MTU})
	logger := log.New(io.Discard, "test", log.LstdFlags)
	api := New(st, cfg, logger, func() {})
	return api
}

func TestNetworkSettingsVersionFlow(t *testing.T) {
	api := testAPI(t)
	srv := api.Serve(":0", "")
	client := newTestClient(t, srv)
	// initial GET
	var ns NetworkSettingsState
	client.doJSON(http.MethodGet, "/api/v1/networks/n1/settings", nil, &ns, 200)
	if ns.Version != 1 {
		t.Fatalf("expected version 1 got %d", ns.Version)
	}
	// update OK
	body := map[string]any{"Version": ns.Version, "mtu": 1500}
	var upd NetworkSettingsState
	client.doJSON(http.MethodPut, "/api/v1/networks/n1/settings", body, &upd, 200)
	// conflict (reuse old version)
	var conflictResp map[string]any
	client.doJSON(http.MethodPut, "/api/v1/networks/n1/settings", body, &conflictResp, 409)
}

func TestMemberPreferencesVersionFlow(t *testing.T) {
	api := testAPI(t)
	srv := api.Serve(":0", "")
	client := newTestClient(t, srv)
	var mp MemberPreferencesState
	client.doJSON(http.MethodGet, "/api/v1/networks/n1/me/preferences", nil, &mp, 200)
	if mp.Version != 1 {
		t.Fatalf("expected version 1 got %d", mp.Version)
	}
	body := map[string]any{"Version": mp.Version, "allow_internet": false}
	var mpUpd MemberPreferencesState
	client.doJSON(http.MethodPut, "/api/v1/networks/n1/me/preferences", body, &mpUpd, 200)
	var mpConflict map[string]any
	client.doJSON(http.MethodPut, "/api/v1/networks/n1/me/preferences", body, &mpConflict, 409)
}

// --- test client helper handling CSRF cookie/header ---
type testClient struct {
	t    *testing.T
	srv  *http.Server
	csrf string
	jar  []*http.Cookie
}

func newTestClient(t *testing.T, srv *http.Server) *testClient {
	return &testClient{t: t, srv: srv}
}

func (c *testClient) doJSON(method, path string, body any, out any, expect int) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if method != http.MethodGet {
		if c.csrf == "" {
			// first do a priming GET to obtain cookie
			priming := httptest.NewRequest(http.MethodGet, "/api/v1/networks/n1/settings", nil)
			rr := httptest.NewRecorder()
			c.srv.Handler.ServeHTTP(rr, priming)
			if rr.Code != 200 {
				c.t.Fatalf("priming get failed %d", rr.Code)
			}
			for _, ck := range rr.Result().Cookies() {
				if ck.Name == "goc_csrf" {
					c.csrf = ck.Value
				}
				c.jar = append(c.jar, ck)
			}
		}
		req.Header.Set("X-CSRF-Token", c.csrf)
		for _, ck := range c.jar {
			req.AddCookie(ck)
		}
	}
	rr := httptest.NewRecorder()
	c.srv.Handler.ServeHTTP(rr, req)
	if rr.Code != expect {
		c.t.Fatalf("%s %s expected %d got %d body=%s", method, path, expect, rr.Code, rr.Body.String())
	}
	if out != nil {
		_ = json.Unmarshal(rr.Body.Bytes(), out)
	}
}
