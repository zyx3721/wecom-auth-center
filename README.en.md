<div align="center">

<h1>wecom-auth-center</h1>

<p><b>WeCom Unified Auth Center</b> — one WeCom app, scan-to-login for N business domains</p>

<p><a href="README.md">简体中文</a> · <b>English</b></p>

A WeCom (WeChat Work) self-built app only gets a handful of trusted/callback domains, while your internal systems live on many different hostnames.
wecom-auth-center funnels WeCom OAuth through a single callback domain: business systems jump in with an `app` identifier,
and after the QR scan users bounce back with a one-time `ticket` to establish their own session.
**The WeCom `secret` lives in exactly one place; adding a new system never touches the WeCom admin console again.**

<p>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white&labelColor=1f2937" alt="Go 1.26+"></a>
  <img src="https://img.shields.io/badge/state%20%2F%20ticket-one--time-059669?labelColor=1f2937" alt="One-time state/ticket">
  <img src="https://img.shields.io/badge/deploy-single%20binary-1f2937?labelColor=1f2937" alt="Single binary">
  <img src="https://img.shields.io/badge/external%20deps-yaml.v3%20only-3B82F6?labelColor=1f2937" alt="yaml.v3 only">
</p>

<p>
  <b><a href="#what-it-solves">What it solves</a></b> ·
  <a href="#how-it-works">How it works</a> ·
  <a href="#tech-stack">Tech stack</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#deployment">Deployment</a> ·
  <a href="#http-api">API</a> ·
  <a href="#data-and-security">Security</a> ·
  <a href="#faq">FAQ</a> ·
  <a href="#repository-layout">Layout</a> ·
  <a href="#documentation">Docs</a>
</p>

</div>

---

Several internal systems (OA, IT helpdesk, CMDB, …) all want "sign in with WeCom", but the WeCom admin console forces you to whitelist callback domains one by one, each requiring domain-ownership verification.
wecom-auth-center applies the battle-tested CAS-style SSO pattern: WeCom only ever trusts `auth.example.com`;
this service receives every OAuth callback, resolves the originating system from `state`, and hands the login result to each system as a one-time `ticket`.
Written in Go with no external database — `go build` produces a single deployable binary — and a built-in mock mode that walks the entire login flow locally without touching WeCom.

## What it solves

- **One callback domain for all systems** — WeCom is configured with `auth.example.com` only; the number of business systems is no longer bounded by the trusted-domain quota.
- **Zero WeCom config per system** — onboarding a new system is one line in the auth center whitelist (`app` id + domain + `app_secret`); the WeCom console stays untouched.
- **`secret` kept in one place** — WeCom `corpid`/`secret` live only in the auth center config; each business system gets an independent `app_secret` used solely for verify signatures.
- **One-time credentials, replay-proof** — `state` (5 min) and `ticket` (60 s) are 128-bit random values consumed on first use; verify checks an HMAC-SHA256 signature and timestamp skew with constant-time comparison.
- **Open-redirect proof** — the `app` → domain mapping exists only in server config; request parameters never carry full URLs, and `redirect` accepts only in-app relative paths.
- **Runs without WeCom** — with `wecom.mock: true` a local mock scan page replaces the real WeCom login, so the full flow is testable in a browser.
- **Production middleware built in** — per-IP rate limiting, request logging, panic recovery, health check and graceful shutdown.

**It is not** an IAM or user-management system: no passwords stored, no permission model, no org chart. WeCom answers "who is this person", this service answers "which system started this login", and each business system answers "what may this person do here".

| Scenario | Per-system WeCom integration | Unified auth center |
| --- | --- | --- |
| Onboarding a system | Edit WeCom console + domain verification | Add one whitelist line |
| Callback domain count | Limited by WeCom per-app quota | Unlimited (WeCom only knows `auth`) |
| WeCom `secret` distribution | One copy per system | Single copy in the auth center |
| Domain verification file | Required for every domain | Required for `auth` only |
| Login code | Each system implements OAuth | Each system implements one ticket check |

## How it works

```text
   oa.example.com      it.example.com      cmdb.example.com
        │                    │                    │
        │ ① 302 /login?app=oa (when not signed in) │
        └──────────┬─────────┴────────────────────┘
                   ▼
      ┌──────────────────────────┐  ② 302 QR page ┌────────────┐
      │   auth.example.com       │ ────────────▶  │   WeCom    │
      │   wecom-auth-center      │ ◀────────────  │  user scans │
      └───────────┬──────────────┘  ③ code        └────────────┘
                  │ ④ code → userid, issue one-time ticket
                  │ ⑤ 302 /sso/login?ticket=xxx (whitelisted domains only)
                  ▼
   business backend POST /api/verify → userid → establish its own session
```

- **`state` routing** — `/login` mints a random state bound to the source `app`; WeCom echoes it back; consumed on first use, giving CSRF and replay protection for free.
- **`ticket` handover** — after exchanging `code` for `userid`, the auth center issues a 60-second one-time ticket and 302s back to the business system.
- **`verify` gate** — the business backend calls `/api/verify` signed with its `app_secret` to redeem the ticket; browsers never touch verify.
- **Session ownership** — the auth center issues no global session; every system establishes its own Session/JWT after verification, and logouts stay independent.
- **Where data lives** — state/ticket are second-lived records kept in process memory by default; a Redis implementation slots into the `Store` interface for multi-instance deployments.

## Tech stack

| Layer | Choice |
| --- | --- |
| Language | Go 1.26+ (standard library `net/http` only, Go 1.22+ method routing) |
| External deps | [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) (the only third-party dependency) |
| WeCom entry | wwlogin QR (PC) / in-app OAuth `snsapi_base` (WeCom browser), config-switchable |
| Storage | In-process memory + TTL (default); Redis behind the `Store` interface when needed |
| Deployment | Single binary + systemd, or a multi-stage Docker image |

## Quick start

Local walkthrough needs only **Go 1.26+** — no WeCom account required:

```bash
git clone <repo-url> wecom-auth-center && cd wecom-auth-center/server
cp config.example.yaml config.yaml
```

Set `wecom.mock` to `true` in `config.yaml` (mock mode skips corpid/secret validation), then:

```bash
go run ./cmd/server -config config.yaml
```

Open `http://127.0.0.1:8700/login?app=oa` in a browser, click the mock "scan succeeded" button,
and watch the 302 chain land on `https://oa.example.com/sso/login?ticket=...`;
redeem the ticket via `POST /api/verify` using the signing recipe in `docs/client-integration.md` to get `{"userid":"mockuser"}`.

Run the tests:

```bash
go test ./...
```

## Deployment

Only two paths are kept: **Docker deployment** (recommended) and **Release binary deployment**. Both require the WeCom console and domain setup first — checklist in [docs/wecom-setup.md](docs/wecom-setup.md); the full configuration reference lives in [docs/manual.md](docs/manual.md) (Chinese).

### Option 1: Docker

The image is built automatically by GitHub CI on each `v*` tag and pushed to **both the Aliyun registry and Docker Hub** (multi-arch amd64/arm64). It contains only the binary and static assets; the config file is bind-mounted from the host. One command is all it takes:

```bash
docker run -d \
  --name wecom-auth-center \
  --restart unless-stopped \
  -p 127.0.0.1:8700:8700 \
  -v /opt/wecom-auth-center/config.yaml:/app/config.yaml:ro \
  registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest
```

Prepare `config.yaml` on the host first (copy from `server/config.example.yaml`, set `wecom.mock: false`, fill in corpid/secret and the app whitelist; the file holds secrets — keep it mode 600). Every field is documented in [docs/manual.md](docs/manual.md).

Service management:

```bash
docker ps --filter name=wecom-auth-center        # status
docker logs -f wecom-auth-center                 # logs
docker restart wecom-auth-center                 # restart
docker stop wecom-auth-center                    # stop
docker stop wecom-auth-center && docker rm wecom-auth-center  # remove container

# Upgrade: pull the new image, then re-create the container with the command above
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest

# Outside mainland China, the Docker Hub mirror of the same image also works (swap the image name for)
docker pull zyx3721/wecom-auth-center:latest
```

**Endpoints**

- Health check: `http://your-host:8700/healthz`
- Login entry (normally reached via business-system redirect): `http://your-host:8700/login?app=oa`

Production must sit behind an Nginx reverse proxy with HTTPS (a hard WeCom requirement) and set `server.trust_proxy: true`; the full HTTP→HTTPS, HSTS sample lives in [docs/manual.md](docs/manual.md).

### Option 2: Release binary

Head to [GitHub Releases](https://github.com/zyx3721/wecom-auth-center/releases), pick the archive for your OS and architecture, then verify, extract, configure and start.

**Which file to download**

| Your machine | File |
| --- | --- |
| Linux x86_64 | `wecom-auth-center_<version>_linux_amd64.tar.gz` |
| Linux ARM64 (Kunpeng, Phytium, …) | `wecom-auth-center_<version>_linux_arm64.tar.gz` |
| macOS Intel | `wecom-auth-center_<version>_darwin_amd64.tar.gz` |
| macOS Apple silicon | `wecom-auth-center_<version>_darwin_arm64.tar.gz` |
| Windows x86_64 | `wecom-auth-center_<version>_windows_amd64.zip` |
| Windows ARM64 | `wecom-auth-center_<version>_windows_arm64.zip` |
| Checksums | `SHA256SUMS` |

Each archive contains the `wecom-auth-center` binary (`.exe` on Windows), `config.example.yaml` and the integration docs. The binary has no external runtime dependencies.

**1. Verify the download**

```bash
VERSION=1.0.0
mkdir -p /opt/wecom-auth-center && cd /opt/wecom-auth-center
sha256sum -c SHA256SUMS
```

**2. Extract**

```bash
tar -xzf wecom-auth-center_${VERSION}_linux_amd64.tar.gz --strip-components=1
```

**3. Configure and start**

```bash
cp config.example.yaml config.yaml
vim config.yaml    # fill in corpid/secret and the app whitelist, mock: false; chmod 600
./wecom-auth-center -config config.yaml
```

The default listen address is `:8700`. For a persistent service, use systemd:

```ini
# /etc/systemd/system/wecom-auth-center.service
[Unit]
Description=wecom-auth-center (WeCom unified auth center)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=wecom-auth
Group=wecom-auth
WorkingDirectory=/opt/wecom-auth-center
ExecStart=/opt/wecom-auth-center/wecom-auth-center -config /opt/wecom-auth-center/config.yaml
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/wecom-auth-center
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now wecom-auth-center
```

**4. Terminate with Nginx**

[deploy/nginx.conf.example](deploy/nginx.conf.example) is a ready-to-use sample (HTTP redirect, HTTPS, HSTS, `X-Real-IP`); a step-by-step walkthrough and troubleshooting live in [docs/manual.md](docs/manual.md).

**5. Endpoints**

Same as Docker: `/healthz`, `/login?app=oa`, `/WW_verify_xxxxxxxx.txt`.

## HTTP API

| Method | Path | Caller | Purpose |
| --- | --- | --- | --- |
| GET | `/login?app=oa&redirect=/path` | Business system (browser redirect) | Whitelist check → register state → 302 to WeCom |
| GET | `/callback?code=&state=` | WeCom | Consume state → code to userid → issue ticket → 302 back |
| POST | `/api/verify` | Business backend | Signature check → one-time ticket redemption → `{userid, name}` |
| GET | `/healthz` | Probes | Returns `ok` |

Verify signature (generated business-side, compared constant-time by the auth center):

```text
sign = hex( HMAC-SHA256( key = app_secret, message = app + "\n" + ticket + "\n" + ts ) )
```

Error codes: `invalid_app` / `invalid_sign` / `expired_ts` / `invalid_ticket`. Full sequence and field reference in [docs/architecture.md](docs/architecture.md) (Chinese); onboarding steps in [docs/client-integration.md](docs/client-integration.md) (Chinese).

## Data and security

```text
WeCom secret stored only in the auth center config.yaml (mode 600, git-ignored)
        +
state / ticket: 128-bit random, one-time consumption, short TTL
        +
verify: HMAC-SHA256 signature + timestamp skew ≤ 60s
        +
whitelist or nothing: unknown app → 400, redirect limited to in-app paths
```

- **HTTPS everywhere** — a hard requirement of WeCom trusted domains; HSTS enabled in the nginx sample.
- **Least-privilege keys** — business systems hold only their own `app_secret`, valid solely for verify signatures.
- **Rate limiting** — `/login` 30/min/IP and `/api/verify` 120/min/IP by default, tunable in config.
- **Audit trail** — failed state checks, ticket replays and signature failures all produce structured logs.
- **Restart semantics** — with in-memory storage a restart drops in-flight logins (users simply rescan); switch to Redis for multi-instance deployments.

## FAQ

**Why not just configure multiple callback domains in WeCom?**

The per-app trusted/callback domain list is small and every entry needs domain-ownership verification; each new system would mean another WeCom console change. With a single callback domain the WeCom side is configured once and never again.

**What is `state` and can it be skipped?**

It is CSRF protection plus source routing: at callback time only the auth center holding the matching state knows where to send the user. Skipping it forces trusting a `redirect` request parameter — an open redirect waiting to happen.

**What if a ticket leaks?**

It expires in 60 seconds, is consumed on first verification, and redemption requires an HMAC signed with the business system's `app_secret`. Keep business-to-auth traffic on HTTPS.

**How do I add a new business system?**

The WeCom console stays untouched. Three steps: register `app`, domain and `app_secret` under `apps:` in the auth center config and restart; add a `/sso/login` route that verifies the ticket; point unauthenticated redirects at `/login?app=<id>`. Details in `docs/client-integration.md`.

**Does it work inside the WeCom client (H5)?**

Yes. Set `wecom.mode: inside` and `/login` switches to in-app OAuth (`snsapi_base`); the rest of the flow is identical. PC QR login remains the default `qrcode`.

**How does a system map the WeCom user to a local account?**

verify returns the WeCom `userid`. Mapping policy is up to each system: admin-provisioned binding (a `wecom_userid` column) is recommended, or auto-bind on first login when `userid == username`.

**Lost a system's `app_secret`?**

It is right there in `apps.<app>.app_secret` in the auth center config; set a new value and restart (the old one dies immediately).

**Can one scan unlock all systems?**

That is the planned phase four: once the auth center issues its own session cookie, already-signed-in users passing `/login` get a ticket without rescanning. Today each system's first login needs a scan.

## Repository layout

```text
wecom-auth-center/
├── server/                     Go service
│   ├── cmd/server/             entrypoint (config loading, graceful shutdown)
│   ├── internal/
│   │   ├── config/             YAML config loading & strict validation
│   │   ├── handler/            /login /callback /api/verify /healthz + routing
│   │   ├── service/            wecom.go (token cache/identity/mock), sso.go (state/ticket)
│   │   ├── store/              short-lived store interface + memory implementation
│   │   └── middleware/         request log, panic recovery, IP rate limiting
│   ├── web/static/             WeCom domain verification file & static assets
│   ├── config.example.yaml     config template (config.yaml is git-ignored)
│   └── go.mod
├── deploy/
│   ├── Dockerfile              multi-stage build (CI pushes to the Aliyun registry)
│   ├── nginx.conf.example      reverse proxy + HTTPS + HSTS for the auth domain
│   └── systemd/wecom-auth-center.service
├── .github/
│   └── workflows/ci.yml        test gate + multi-arch tag release + image push
├── docs/                       architecture, WeCom setup, integration guide & manual (Chinese)
├── AGENTS.md                   AI-assistant development conventions (Chinese)
├── README.md                   简体中文
└── README.en.md                English (this file)
```

## Documentation

| Start here | Then |
| --- | --- |
| [Quick start](#quick-start) | Local mock walkthrough, no WeCom needed |
| [Deployment](#deployment) | Docker and Release binary paths, image registry, reverse proxy |
| [HTTP API](#http-api) | Callers of the four endpoints and the signing recipe |
| docs/manual.md (Chinese) | Full manual: config reference, API debugging, Nginx/HTTPS, upgrade & troubleshooting |
| docs/architecture.md (Chinese) | Full sequence diagram, data & security design |
| docs/wecom-setup.md (Chinese) | WeCom console and domain checklist |
| docs/client-integration.md (Chinese) | Business-system onboarding guide |
| [README.md](README.md) | 同样的内容，中文版 |
