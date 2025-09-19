# GOConnect Development Guide

This guide helps contributors get productive in ~15 minutes with a working dev environment, a local smoke test, and awareness of build, run, security, and testing flows.

## 0) Prerequisites

- Windows 10/11
- Go 1.23+
- PowerShell (default on Windows)
- Optional: Wintun driver/DLL if you want to exercise real TUN (not required for basic dev)

## 1) Clone and Build

```powershell
git clone https://github.com/orhaniscoding/GOConnect.git
cd GOConnect
go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller
go build -o bin/goconnect-service.exe ./cmd/goconnectservice
# (optional) compile-only Wintun path
go build -tags=wintun ./...
```

## 2) First Run (Dev Mode)

Start the service in the foreground and open the Web UI:

```powershell
go run ./cmd/goconnectservice
```

Open: http://127.0.0.1:2537

Logs are written to `%ProgramData%/GOConnect/logs/agent.log` (JSON by default).

## 3) Configure

The first run creates `%ProgramData%/GOConnect/config/config.yaml` with defaults. Edit as needed:

```yaml
api:
	auth: bearer
	bearer_token: "changeme-local-owner"
	rate_limit: { rps: 10, burst: 20 }
	validation: true
logging: { format: json, level: info }
metrics: { enabled: false, addr: 127.0.0.1:9090 }
stun_servers: ["stun.l.google.com:19302"]
```

Notes:
- Mutating POST calls require CSRF via `X-CSRF-Token` header (cookie `goc_csrf`).
- If you set `controller_url`, the agent will proxy `/api/controller/*` to that URL (and attach the bearer token from `secrets/controller_token.txt` when present).

## 4) Smoke Tests

Diagnostics (STUN + MTU):

```powershell
bin/goconnect-service.exe diag
```

Expected JSON with `stun_ok` and `mtu_ok` true on a typical setup.

API Diagnostic via PowerShell (CSRF + Bearer):

```powershell
Invoke-WebRequest http://127.0.0.1:2537/api/status -OutFile $null -SessionVariable S
$csrf = $S.Cookies.GetCookies('http://127.0.0.1:2537')['goc_csrf'].Value
Invoke-RestMethod -Method Post http://127.0.0.1:2537/api/diag/run -Headers @{ 'Authorization'='Bearer changeme-local-owner'; 'X-CSRF-Token'=$csrf }
```

Optional TUN compile smoke:

```powershell
go build -tags=wintun ./...
```

## 5) Updater (Dev)

Enable in config to test the flow:

```yaml
updater:
	enabled: true
	repo: "orhaniscoding/GOConnect"
```

- Check:
```powershell
bin/goconnect-service.exe self-update --check
```
- Apply (staged):
```powershell
bin/goconnect-service.exe self-update
```

The new binary is downloaded as `.new`; if possible, an atomic swap with `.bak` is performed. If locked, a `.update-staged` marker is written for activation on restart.

## 6) Security Notes (Dev)

- Keep `api.bearer_token` private. The API is designed for local admin by default.
- CSRF is enforced for POSTs via `X-CSRF-Token`.
- Windows DPAPI is available for machine-bound secret encryption in `internal/security`.
- Trusted peer certs support file paths or inline PEM blocks.

## 7) Controller Store

Controller defaults to SQLite. Override via env for fast iteration:

```powershell
$env:GOCONNECT_STORE_TYPE = 'memory'   # or 'sqlite' (default)
$env:GOCONNECT_DATA_DIR = '.\data'
bin/goconnectcontroller.exe
```

## 8) Tests & Lint

Run tests:

```powershell
go test ./... -count=1
```

With coverage:

```powershell
go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Lint and vet locally:

```powershell
go vet ./...
golangci-lint run --timeout=5m
```

CI enforces:
- vet + golangci-lint
- tests with combined coverage; gate at ≥60%
- Windows Wintun compile-only job
- optional Linux race test

## 9) VS Code Tips

Tasks available in this repo:
- Build Service, Build All, Test (short), Run Service (dev), Run Service (wintun)

Debug the service via `cmd/goconnectservice/main.go` launch. Set breakpoints anywhere under `internal/*`.

## 10) Troubleshooting

- HTTP API not starting: ensure port 2537 is free, check `%ProgramData%/GOConnect/logs/agent.log` for JSON errors.
- CSRF failures: perform a GET to `/api/status` to obtain the `goc_csrf` cookie before POSTs.
- STUN failures: verify outbound UDP; try another server like `stun1.l.google.com:19302`.
- Wintun issues: for compile-only checks use `-tags=wintun` without needing the driver; to actually use the TUN device, run as Administrator and ensure Wintun is installed.

Happy hacking!
