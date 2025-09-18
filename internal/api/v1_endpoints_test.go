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
			// first do a priming GET to obtain cookie using a stable endpoint
			priming := httptest.NewRequest(http.MethodGet, "/api/status", nil)
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

func TestJoinSecretValidation(t *testing.T) {
	// Start with empty config (no preexisting networks) to test setting secret on first join
	cfg := config.Default("en")
	st := core.NewState(core.Settings{Port: cfg.Port, MTU: cfg.MTU})
	logger := log.New(io.Discard, "test", log.LstdFlags)
	api := New(st, cfg, logger, func() {})
	srv := api.Serve(":0", "")
	client := newTestClient(t, srv)

	// 1) First join with secret provided -> should succeed and persist secret
	var joinOK map[string]any
	body1 := map[string]any{"id": "secnet", "name": "SecNet", "join_secret": "s3cr3t"}
	client.doJSON(http.MethodPost, "/api/networks/join", body1, &joinOK, 200)

	// 2) Second join without providing secret -> should error 400 missing_join_secret
	var joinMissing map[string]any
	body2 := map[string]any{"id": "secnet"}
	// Use client helper to set csrf/cookies properly
	client.doJSON(http.MethodPost, "/api/networks/join", body2, &joinMissing, 400)

	// 3) Second join with wrong secret -> 403 invalid_join_secret
	var joinWrong map[string]any
	body3 := map[string]any{"id": "secnet", "join_secret": "wrong"}
	client.doJSON(http.MethodPost, "/api/networks/join", body3, &joinWrong, 403)

	// 4) Correct secret -> 200
	var joinOK2 map[string]any
	body4 := map[string]any{"id": "secnet", "join_secret": "s3cr3t"}
	client.doJSON(http.MethodPost, "/api/networks/join", body4, &joinOK2, 200)
}
