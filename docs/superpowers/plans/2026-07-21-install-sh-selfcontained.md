# install.sh 自包含安装脚本 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ticket-based dynamic install.sh + `aris trace init` wizard with a self-contained POSIX sh install script served at `GET /install.sh`, with binaries distributed via GitHub Releases.

**Architecture:** Server generates install.sh with `{{.Host}}` embedded via `text/template` + `//go:embed`. The script downloads the aris binary from GitHub Releases, then runs a 4-step interactive wizard (health check, agent select, API key, hook setup) entirely in sh. The aris binary keeps only `trace ingest`. The ticket system, download endpoint, and artifact resolver are removed.

**Tech Stack:** Go (huma/fiber router, text/template, embed), POSIX sh (install script), jq (JSON manipulation), GitHub Actions/Releases (binary distribution), React/Next.js (web UI).

## Global Constraints

- POSIX sh compatibility (`#!/bin/sh`), not bash-specific
- Interactive reads must use `/dev/tty` (stdin is the curl pipe)
- Hidden input via `stty -echo` with graceful fallback
- File permissions: `~/.aris/` (0700), `~/.aris/bin/` (0700), `~/.aris/trace/config.json` (0600), `~/.codex/hooks.json` (0600)
- API Key never appears in script content, command args, or shell history
- Binary download URL: `https://github.com/hcd233/aris-proxy-api/releases/latest/download/aris-<os>-<arch>`
- 10 hook events: SessionStart, UserPromptSubmit, PreToolUse, PermissionRequest, PostToolUse, Stop, SubagentStart, SubagentStop, PreCompact, PostCompact
- Hook group format: `{"matcher":"","hooks":[{"type":"command","command":"<path> trace ingest","timeout":30}]}`
- config.json format: `{"host":"<host>","agent":"codex","apiKey":"<key>"}` (unchanged)
- `trace ingest` logic, spool, rollout, state — completely unchanged
- Go lint: `go run ./cmd/server lint conv ./...` must pass
- Go tests: `go test ./...` must pass

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/handler/install_trace_client.sh.tmpl` | Create | Embedded POSIX sh install script template |
| `internal/handler/trace.go` | Modify | Add `HandleInstallScript`; remove old handlers in Task 2 |
| `internal/dto/trace.go` | Modify | Add `InstallScriptReq`; remove old DTOs in Task 2 |
| `internal/router/router.go` | Modify | Register `GET /install.sh`; remove `TraceClientTicketStore` dep |
| `internal/router/trace.go` | Modify | Remove ticket/download/install routes |
| `internal/application/trace/port/handler.go` | Modify | Remove ticket/artifact port interfaces |
| `internal/application/trace/command/issue_client_ticket.go` | Delete | Ticket issuance command |
| `internal/infrastructure/cache/trace_client_ticket.go` | Delete | Redis ticket store |
| `internal/infrastructure/traceclient/artifact*.go` | Delete | Artifact resolver |
| `internal/middleware/trace_client_ticket.go` | Delete | Ticket middleware |
| `internal/bootstrap/modules/repository.go` | Modify | Remove ticket store + artifact resolver providers |
| `internal/bootstrap/modules/application.go` | Modify | Remove ticket handler provider |
| `internal/bootstrap/modules/handler.go` | Modify | Remove ticket/artifact deps from `TraceDependencies` |
| `internal/common/constant/traceclient.go` | Modify | Remove ticket/artifact/init constants |
| `internal/common/constant/ratelimit.go` | Modify | Remove ticket rate limit constants |
| `env/api.env.template` | Modify | Remove `TRACE_CLIENT_ARTIFACT_DIR` |
| `cmd/client/trace.go` | Modify | Remove `trace init` command |
| `internal/tracecli/init.go` | Delete | InitRunner |
| `internal/tracecli/codex.go` | Delete | CodexHookInstaller |
| `internal/tracecli/terminal.go` | Delete | Terminal |
| `internal/tracecli/config.go` | Modify | Remove `Save`, keep `Load` |
| `internal/tracecli/http_client.go` | Modify | Remove `CheckHealth`, `CheckAPIKey` |
| `web/src/components/trace-install-dialog.tsx` | Modify | Simplify to `curl \| sh` |
| `web/src/lib/api-client.ts` | Modify | Remove `issueTraceClientTicket` |
| `web/src/lib/types.ts` | Modify | Remove `IssueTraceClientTicketRsp` |
| `web/src/locales/{en,zh,ja}.json` | Modify | Update i18n strings |
| `.github/workflows/release-trace-client.yml` | Create | GitHub Releases workflow |
| `docker/dockerfile` | Modify | Remove cross-compile step |
| `test/e2e/trace/client_download_test.go` | Modify | Update tests |

---

## Task 1: Create install.sh Template + Handler + Route

**Files:**
- Create: `internal/handler/install_trace_client.sh.tmpl`
- Modify: `internal/handler/trace.go` (add `HandleInstallScript` to interface + impl)
- Modify: `internal/dto/trace.go` (add `InstallScriptReq`)
- Modify: `internal/router/router.go` (register `GET /install.sh`)
- Test: `test/e2e/trace/install_script_test.go`

**Interfaces:**
- Produces: `TraceHandler.HandleInstallScript(ctx, *dto.InstallScriptReq) (*huma.StreamResponse, error)` — new method on `TraceHandler` interface
- Produces: `dto.InstallScriptReq` — empty request struct for huma route registration

- [ ] **Step 1: Create the install.sh template file**

Create `internal/handler/install_trace_client.sh.tmpl`:

```
#!/bin/sh
# Aris trace client installer — self-contained, no ticket required.
# Generated by aris-proxy-api; server host is embedded below.
set -eu

host='{{.Host}}'
gh_base='https://github.com/hcd233/aris-proxy-api/releases/latest/download'

# --- preflight ---
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v jq   >/dev/null 2>&1 || { echo "jq is required (brew install jq / apt install jq)" >&2; exit 1; }

# --- platform detection ---
case "$(uname -s)-$(uname -m)" in
  Darwin-x86_64)             os=darwin; arch=amd64 ;;
  Darwin-arm64)              os=darwin; arch=arm64 ;;
  Linux-x86_64)              os=linux;  arch=amd64 ;;
  Linux-aarch64|Linux-arm64) os=linux;  arch=arm64 ;;
  *) echo "Unsupported platform: $(uname -s)/$(uname -m)" >&2; exit 1 ;;
esac

# --- download binary ---
aris_bin="$HOME/.aris/bin/aris"
mkdir -p -m 0700 "$(dirname "$aris_bin")"
tmp=$(mktemp "${TMPDIR:-/tmp}/aris.XXXXXX")
trap 'rm -f "$tmp"' EXIT
echo "Downloading aris-$os-$arch..."
curl -fsSL -o "$tmp" "$gh_base/aris-$os-$arch"
chmod 0700 "$tmp"
mv "$tmp" "$aris_bin"
trap - EXIT
echo "Installed to $aris_bin"

# open /dev/tty for interactive prompts (stdin is the curl pipe)
exec 3<>/dev/tty

# --- [1/4] connect to server ---
echo "[1/4] Connect to server"
while :; do
  if curl -sf --max-time 5 "$host/health" >/dev/null 2>&1; then
    echo "Connected"
    break
  fi
  printf "Connection failed. Retry? [Y/n]: " >&3
  IFS= read -r answer <&3 || exit 1
  case "$answer" in n|N|no|NO) exit 1 ;; esac
done

# --- [2/4] select agent ---
echo "[2/4] Select agent"
while :; do
  printf "Press Enter to select Codex: " >&3
  IFS= read -r answer <&3 || exit 1
  case "$answer" in
    ""|codex|Codex) break ;;
    *) echo "Only Codex is supported" >&3 ;;
  esac
done

# --- [3/4] configure API key ---
echo "[3/4] Configure API key"
config_file="$HOME/.aris/trace/config.json"
existing_key=""
if [ -f "$config_file" ]; then
  existing_key=$(jq -r '.apiKey // ""' "$config_file" 2>/dev/null || true)
fi

while :; do
  if [ -n "$existing_key" ]; then
    printf "API key (Enter keeps current): " >&3
  else
    printf "API key: " >&3
  fi
  stty -echo <&3 2>/dev/null || true
  IFS= read -r api_key <&3 || { stty echo <&3 2>/dev/null || true; exit 1; }
  stty echo <&3 2>/dev/null || true
  echo "" >&3
  [ -z "$api_key" ] && api_key="$existing_key"
  [ -z "$api_key" ] && { echo "api key is required" >&3; continue; }
  if curl -sf --max-time 5 -H "Authorization: Bearer $api_key" \
      "$host/api/v1/trace/client/check" >/dev/null 2>&1; then
    break
  fi
  echo "API key validation failed" >&3
  printf "Retry API key? [Y/n]: " >&3
  IFS= read -r retry <&3 || exit 1
  case "$retry" in n|N|no|NO) exit 1 ;; esac
done

# --- [4/4] configure Codex hooks ---
echo "[4/4] Configure Codex hooks"
codex_dir="$HOME/.codex"
codex_hooks="$codex_dir/hooks.json"
hook_cmd="$aris_bin trace ingest"
aris_group='{"matcher":"","hooks":[{"type":"command","command":"'"$hook_cmd"'","timeout":30}]}'

mkdir -p -m 0700 "$codex_dir"

if [ -f "$codex_hooks" ]; then
  cp "$codex_hooks" "$codex_hooks.bak"
  chmod 600 "$codex_hooks.bak"
  config=$(cat "$codex_hooks")
else
  config='{}'
fi

for event in SessionStart UserPromptSubmit PreToolUse PermissionRequest \
             PostToolUse Stop SubagentStart SubagentStop PreCompact PostCompact; do
  config=$(printf '%s' "$config" | jq \
    --arg event "$event" \
    --arg cmd "$hook_cmd" \
    --argjson group "$aris_group" \
    '.hooks[$event] = ((.hooks[$event] // [])
      | map(select(any(.hooks[]?; .command == $cmd) | not))
      + [$group])')
done

tmp_config=$(mktemp "${TMPDIR:-/tmp}/aris-hooks.XXXXXX")
printf '%s\n' "$config" | jq . > "$tmp_config"
chmod 600 "$tmp_config"
mv "$tmp_config" "$codex_hooks"
echo "Codex hooks configured"

# --- write config.json ---
trace_dir="$HOME/.aris/trace"
mkdir -p -m 0700 "$trace_dir"
tmp_config=$(mktemp "${TMPDIR:-/tmp}/aris-config.XXXXXX")
jq -n --arg host "$host" --arg agent "codex" --arg key "$api_key" \
  '{host:$host, agent:$agent, apiKey:$key}' > "$tmp_config"
chmod 600 "$tmp_config"
mv "$tmp_config" "$config_file"

exec 3>&-

# --- done ---
echo ""
echo "Trace configuration completed"
echo "Config: $config_file"
echo ""
echo "In Codex, run /hooks and manually approve the new Aris hooks."
```

- [ ] **Step 2: Add `InstallScriptReq` DTO**

In `internal/dto/trace.go`, add after `InstallTraceClientReq` (line ~113):

```go
// InstallScriptReq 安装脚本请求（无鉴权，服务端嵌入 host）。
type InstallScriptReq struct{}
```

- [ ] **Step 3: Add `HandleInstallScript` to `TraceHandler` interface**

In `internal/handler/trace.go`, add to the `TraceHandler` interface (after `HandleInstallTraceClient`, line ~40):

```go
	HandleInstallScript(ctx context.Context, req *dto.InstallScriptReq) (*huma.StreamResponse, error)
```

- [ ] **Step 4: Implement `HandleInstallScript`**

In `internal/handler/trace.go`, add the embed directive and import at the top of the file. Add `"bytes"`, `"embed"`, and `"text/template"` to the import block. Add after the imports:

```go
//go:embed install_trace_client.sh.tmpl
var installScriptTemplate string

var installScriptTmpl = template.Must(template.New("install").Parse(installScriptTemplate))

type installScriptData struct {
	Host string
}
```

Then add the handler method (after `writeInstallScriptError`):

```go
// HandleInstallScript 返回自包含的安装脚本（无票据，host 从请求头推导）。
func (h *traceHandler) HandleInstallScript(
	_ context.Context,
	_ *dto.InstallScriptReq,
) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(humaCtx huma.Context) {
		scheme := humaCtx.Header(constant.HTTPHeaderXForwardedProto)
		if scheme == "" {
			scheme = constant.HTTPSchemeHTTP
		}
		origin := scheme + "://" + humaCtx.Header(constant.HTTPHeaderHost)

		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != constant.HTTPSchemeHTTP && parsed.Scheme != constant.HTTPSchemeHTTPS) || parsed.Host == "" {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Invalid origin for install script",
				zap.String("origin", origin),
			)
			writeInstallScriptError(humaCtx, "Failed to determine server origin.")
			return
		}

		var buf bytes.Buffer
		if err := installScriptTmpl.Execute(&buf, installScriptData{Host: origin}); err != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Failed to execute install script template",
				zap.Error(err),
			)
			writeInstallScriptError(humaCtx, "Failed to generate install script.")
			return
		}

		humaCtx.SetStatus(fiber.StatusOK)
		humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeTextPlain)
		humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
		if _, writeErr := io.WriteString(humaCtx.BodyWriter(), buf.String()); writeErr != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[TraceHandler] Failed to write install script",
				zap.Error(writeErr),
			)
		}
	}}, nil
}
```

- [ ] **Step 5: Register `GET /install.sh` route**

In `internal/router/router.go`, inside `RegisterAPIRouter`, add after `initHealthRouter` call (line ~75):

```go
	huma.Register(humaAPI, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Summary:     "InstallTraceScript",
		Description: "Return the self-contained Aris trace client install script",
		Tags:        []string{constant.TagTrace},
	}, deps.TraceHandler.HandleInstallScript)
```

- [ ] **Step 6: Write the failing test**

Create `test/e2e/trace/install_script_test.go`:

```go
package trace_e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

func TestInstallScript_ReturnsScriptWithHost(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	req.Host = "aris.example.com"
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "https")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get(constant.HTTPHeaderContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain, got %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	script := string(body)
	if !strings.Contains(script, "https://aris.example.com") {
		t.Fatalf("script must contain embedded host, got:\n%s", script)
	}
	if !strings.Contains(script, "github.com/hcd233/aris-proxy-api/releases/latest/download") {
		t.Fatalf("script must contain GitHub Releases URL")
	}
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Fatalf("script must start with #!/bin/sh")
	}
}

func TestInstallScript_InvalidSchemeReturnsErrorScript(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	req.Host = ""
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "ftp")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "exit 1") {
		t.Fatalf("invalid origin should return error script, got:\n%s", body)
	}
}
```

- [ ] **Step 7: Run test to verify it fails (compile errors expected)**

Run: `go test ./test/e2e/trace/ -run TestInstallScript -v 2>&1 | head -20`
Expected: FAIL — `HandleInstallScript` not yet on interface / compile error

- [ ] **Step 8: Verify it compiles and tests pass**

Run: `go build ./internal/handler/ && go test ./test/e2e/trace/ -run TestInstallScript -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/handler/install_trace_client.sh.tmpl internal/handler/trace.go internal/dto/trace.go internal/router/router.go test/e2e/trace/install_script_test.go
git commit -m "feat: add self-contained install.sh endpoint at GET /install.sh"
```

---

## Task 2: Backend Cleanup — Remove Ticket/Download/Old Install Code

**Files:**
- Modify: `internal/router/trace.go` — remove ticket/download/install routes + `TicketStore` dep
- Modify: `internal/router/router.go` — remove `TraceClientTicketStore` from deps
- Modify: `internal/handler/trace.go` — remove `HandleIssueTraceClientTicket`, `HandleDownloadTraceClient`, `HandleInstallTraceClient`, `buildInstallScript`; remove `issueTicket`, `artifactResolver`, `ticketStore` fields; remove from interface + `TraceDependencies`
- Modify: `internal/dto/trace.go` — remove `IssueTraceClientTicketReq/Rsp`, `DownloadTraceClientReq`, `InstallTraceClientReq`
- Modify: `internal/application/trace/port/handler.go` — remove `TraceClientTicketStore`, `IssueTraceClientTicket*`, `TraceClientArtifact*`
- Delete: `internal/application/trace/command/issue_client_ticket.go`
- Delete: `internal/infrastructure/cache/trace_client_ticket.go`
- Delete: `internal/infrastructure/traceclient/` (artifact resolver files)
- Delete: `internal/middleware/trace_client_ticket.go`
- Modify: `internal/bootstrap/modules/repository.go` — remove `NewTraceClientTicketStore`, `NewTraceClientArtifactResolver`
- Modify: `internal/bootstrap/modules/application.go` — remove `NewIssueTraceClientTicketHandler`
- Modify: `internal/bootstrap/modules/handler.go` — remove `IssueTicket`, `ArtifactResolver`, `TicketStore` from `NewTraceDependencies`
- Modify: `internal/common/constant/traceclient.go` — remove ticket/artifact/init constants
- Modify: `internal/common/constant/ratelimit.go` — remove ticket rate limit constants
- Modify: `env/api.env.template` — remove `TRACE_CLIENT_ARTIFACT_DIR`
- Modify: `test/e2e/trace/client_download_test.go` — rewrite to only test `HandleInstallScript` (already covered in Task 1, so delete old tests)
- Test: verify compilation + remaining tests pass

**Interfaces:**
- Consumes: `TraceHandler.HandleInstallScript` from Task 1 (must remain)
- Consumes: `writeInstallScriptError` (kept — used by `HandleInstallScript`)
- Produces: cleaned `TraceHandler` interface without ticket/download/install methods

- [ ] **Step 1: Remove routes from `internal/router/trace.go`**

Remove the `issueTraceClientTicket` route registration (lines 59-73), the `downloadGroup` block (lines 93-99), and the `installTraceClient` route registration (lines 101-108). Remove `TicketStore` from `TraceRouterDependencies`:

```go
type TraceRouterDependencies struct {
	TraceHandler handler.TraceHandler
}
```

Update the call site — remove `TicketStore` from the struct literal.

- [ ] **Step 2: Remove `TraceClientTicketStore` from `APIRouterDependencies`**

In `internal/router/router.go`, remove the `TraceClientTicketStore` field from `APIRouterDependencies` struct and the `TicketStore` line from the `initTraceRouter` call.

- [ ] **Step 3: Remove handler methods + interface entries + deps**

In `internal/handler/trace.go`:
- Remove from `TraceHandler` interface: `HandleIssueTraceClientTicket`, `HandleDownloadTraceClient`, `HandleInstallTraceClient`
- Remove from `TraceDependencies`: `IssueTicket`, `ArtifactResolver`, `TicketStore`
- Remove from `traceHandler` struct: `issueTicket`, `artifactResolver`, `ticketStore`
- Remove from `NewTraceHandler`: corresponding assignments
- Delete method implementations: `HandleIssueTraceClientTicket`, `HandleDownloadTraceClient`, `HandleInstallTraceClient`, `buildInstallScript`
- Keep: `writeInstallScriptError` (used by `HandleInstallScript` from Task 1), `HandleCheckTraceClient`, `HandleReportTraceEvent`, list/get/events/conversation handlers, `HandleInstallScript`
- Remove now-unused imports: `os`, `regexp`, `strconv`, `sonic`, `lo` if no longer used

- [ ] **Step 4: Remove DTOs**

In `internal/dto/trace.go`, delete: `IssueTraceClientTicketReq`, `IssueTraceClientTicketRsp`, `DownloadTraceClientReq`, `InstallTraceClientReq`. Keep `InstallScriptReq` and `CheckTraceClientReq`.

- [ ] **Step 5: Remove port interfaces**

In `internal/application/trace/port/handler.go`, delete: `TraceClientTicketStore`, `IssueTraceClientTicketCommand`, `TraceClientTicketView`, `IssueTraceClientTicketHandler`, `TraceClientArtifact`, `TraceClientArtifactResolver`. Remove the `time` import if no longer used.

- [ ] **Step 6: Delete infrastructure files**

```bash
rm internal/application/trace/command/issue_client_ticket.go
rm internal/infrastructure/cache/trace_client_ticket.go
rm internal/middleware/trace_client_ticket.go
```

For the artifact resolver, find and delete the relevant files in `internal/infrastructure/traceclient/`:

```bash
ls internal/infrastructure/traceclient/
# Delete the artifact resolver file(s) — keep the directory if other files exist, otherwise delete it
```

- [ ] **Step 7: Remove DI providers**

In `internal/bootstrap/modules/repository.go`:
- Remove `NewTraceClientTicketStore` and `NewTraceClientArtifactResolver` from the fx.Provide list and delete the functions.
- Remove imports: `cache` (if only used for ticket store), `traceclient`, `config` (if only used for artifact dir).

In `internal/bootstrap/modules/application.go`:
- Remove `NewIssueTraceClientTicketHandler` from the fx.Provide list and delete the function.
- Remove the import of `tracecommand` if no longer used.

In `internal/bootstrap/modules/handler.go`:
- Remove `issueTicket`, `artifactResolver`, `ticketStore` parameters from `NewTraceDependencies`.
- Remove the corresponding field assignments.

- [ ] **Step 8: Remove constants**

In `internal/common/constant/traceclient.go`, delete:
- All `TraceClientArtifact*` constants (lines 12-19)
- All `TraceClientTicket*` constants (lines 20-26)
- `TraceClientInit*` constants (lines 74-93)
- `TraceClientInstallErrorMessage` (line 75)
- `TraceClientCheckPath` (line 49, only used by deleted `CheckAPIKey`)
- `TraceClientHookTypeCommand`, `TraceClientHooksField`, `TraceClientCodexDirName`, `TraceClientCodexHooksFile`, `TraceClientCodexBackupSuffix`, `TraceClientIngestCommandSuffix`, `TraceClientHookTimeout` (lines 39-45, used only by deleted `codex.go`)
- `TraceClientSchemeHTTP`, `TraceClientSchemeHTTPS` (lines 47-48, used by deleted `init.go` — BUT check if `HandleInstallScript` uses them: yes it does! Keep these.)
- `TraceClientNegativeShort`, `TraceClientNegative`, `TraceClientJSONIndent` (lines 92-94, used by deleted init.go/config.go Save)

**Important:** Keep `TraceClientSchemeHTTP` and `TraceClientSchemeHTTPS` — they are used by `HandleInstallScript`.

In `internal/common/constant/ratelimit.go`, delete `PeriodIssueTraceClientTicket` and `LimitIssueTraceClientTicket`.

- [ ] **Step 9: Remove config**

In `env/api.env.template`, remove the `TRACE_CLIENT_ARTIFACT_DIR` line. Search for the config struct field in the config package and remove it:

```bash
grep -rn "TraceClientArtifactDir" internal/
```

Remove the config field and its Viper mapping.

- [ ] **Step 10: Update E2E tests**

Rewrite `test/e2e/trace/client_download_test.go` — delete the old ticket/download test functions. The file may be empty or contain only the `TestInstallScript_*` tests from Task 1 (which live in a separate file). If the file has no remaining tests, delete it:

```bash
rm test/e2e/trace/client_download_test.go
```

- [ ] **Step 11: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 12: Run remaining tests**

Run: `go test ./... 2>&1 | tail -20`
Expected: All pass (some tests may have been removed, that's OK)

- [ ] **Step 13: Run lint**

Run: `go run ./cmd/server lint conv ./...`
Expected: Pass

- [ ] **Step 14: Commit**

```bash
git add -A
git commit -m "refactor: remove ticket system, download endpoint, and old install handler

Binary distribution moves to GitHub Releases. Ticket store, middleware,
artifact resolver, and all related code deleted. install.sh is now served
at GET /install.sh without auth."
```

---

## Task 3: Client Binary — Remove `trace init`

**Files:**
- Modify: `cmd/client/trace.go` — remove `newTraceInitCommand`, keep only `trace ingest`
- Delete: `internal/tracecli/init.go`
- Delete: `internal/tracecli/codex.go`
- Delete: `internal/tracecli/terminal.go`
- Modify: `internal/tracecli/config.go` — remove `ConfigStore.Save` and `writePrivateFile` (if only used by Save)
- Modify: `internal/tracecli/http_client.go` — remove `CheckHealth`, `CheckAPIKey`
- Modify: `internal/common/constant/traceclient.go` — remove init-related constants (already done in Task 2 for some; clean up any remaining)
- Test: verify `cmd/client` compiles, `trace ingest` path unchanged

**Interfaces:**
- Consumes: none (this is a removal task)
- Produces: `aris` binary with only `trace ingest` command

- [ ] **Step 1: Remove `trace init` from `cmd/client/trace.go`**

Replace the entire `newTraceCommand` function to only register `trace ingest`:

```go
func newTraceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "trace", Short: "Ingest agent traces"}
	cmd.AddCommand(newTraceIngestCommand())
	return cmd
}
```

Delete the `newTraceInitCommand` function entirely. Remove unused imports (`os`, `tracecli.InitRunner`, etc. — keep only what `newTraceIngestCommand` needs).

- [ ] **Step 2: Delete init-only files**

```bash
rm internal/tracecli/init.go
rm internal/tracecli/codex.go
rm internal/tracecli/terminal.go
```

- [ ] **Step 3: Modify `internal/tracecli/config.go`**

Remove `ConfigStore.Save` method and `writePrivateFile` function (if only used by Save). Keep `ConfigStore.Load`, `Config` struct, and `configStore` struct. Remove unused imports (`errors`, `filepath` if no longer needed — check if `writePrivateFile` is the only user).

The file should retain:
- `Config` struct
- `ConfigStore` interface with only `Load`
- `configStore` struct
- `NewConfigStore` constructor
- `Load` method

- [ ] **Step 4: Modify `internal/tracecli/http_client.go`**

Remove `CheckHealth` and `CheckAPIKey` methods. Keep the `HTTPClient` struct, `NewHTTPClient` constructor, and any POST method used by `ingest.go`.

- [ ] **Step 5: Verify client compiles**

Run: `go build ./cmd/client/`
Expected: No errors

- [ ] **Step 6: Run client-related tests**

Run: `go test ./internal/tracecli/...`
Expected: Pass (init-related tests should have been deleted with the files)

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove trace init from aris client binary

Configuration is now handled by install.sh. The aris binary only
retains trace ingest (hook callback). Deletes init.go, codex.go,
terminal.go; trims config.go and http_client.go."
```

---

## Task 4: Web UI — Simplify Install Dialog

**Files:**
- Modify: `web/src/components/trace-install-dialog.tsx`
- Modify: `web/src/lib/api-client.ts` — remove `issueTraceClientTicket`
- Modify: `web/src/lib/types.ts` — remove `IssueTraceClientTicketRsp`
- Modify: `web/src/locales/en.json`, `web/src/locales/zh.json`, `web/src/locales/ja.json`

**Interfaces:**
- Consumes: none
- Produces: simplified install dialog showing `curl -fsSL <host>/install.sh | sh`

- [ ] **Step 1: Simplify `trace-install-dialog.tsx`**

Rewrite the component to remove ticket logic. The key changes:
- Remove `issueTraceClientTicket` import and call
- Remove `generateInstallCommand`, `shellQuote`, `TICKET_PLACEHOLDER`
- Remove `copying` state (copy is now synchronous)
- Command is `curl -fsSL <host>/install.sh | sh`
- Steps simplified to 2: download+configure, approve hooks

Replace the entire file content with:

```tsx
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Codex } from "@lobehub/icons";
import { Check, Copy, ShieldCheck, Terminal, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useT } from "@/lib/i18n";

interface TraceInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function generateInstallCommand(hostValue: string): string {
  const host = hostValue.replace(/\/$/, "");
  return `curl -fsSL ${host}/install.sh | sh`;
}

hljs.registerLanguage("bash", bash);

const CODE_SYNTAX =
  "[&_.hljs-comment]:text-[#8C857B] [&_.hljs-comment]:italic " +
  "[&_.hljs-keyword]:text-[#C77B5A] [&_.hljs-built_in]:text-[#7DA1C4] " +
  "[&_.hljs-string]:text-[#9CB071] [&_.hljs-number]:text-[#D69A6B] " +
  "[&_.hljs-literal]:text-[#D69A6B] [&_.hljs-attr]:text-[#7DA1C4] " +
  "[&_.hljs-title]:text-[#7DA1C4] [&_.hljs-params]:text-[#E5E0D6] " +
  "[&_.hljs-variable]:text-[#D69A6B] [&_.hljs-operator]:text-[#9FB3C2] " +
  "[&_.hljs-punctuation]:text-[#A8A296] [&_.hljs-property]:text-[#7DA1C4]";

export default function TraceInstallDialog({
  open,
  onOpenChange,
}: TraceInstallDialogProps) {
  const t = useT();
  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const [host] = useState(() =>
    typeof window === "undefined" ? "" : window.location.origin
  );
  const [copied, setCopied] = useState(false);

  const previewCommand = useMemo(
    () => generateInstallCommand(host || "https://your-aris-server.example"),
    [host]
  );
  const highlighted = useMemo(
    () => hljs.highlight(previewCommand, { language: "bash" }).value,
    [previewCommand]
  );

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(generateInstallCommand(host));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* noop */
    }
  }, [host]);

  const handleClose = useCallback(() => {
    setCopied(false);
    onOpenChange(false);
  }, [onOpenChange]);

  useEffect(() => {
    if (open && closeBtnRef.current) {
      setTimeout(() => closeBtnRef.current?.focus(), 120);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
      >
        <DialogHeader className="shrink-0 flex-row items-center gap-3 border-b border-border px-6 py-4">
          <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
            <Codex.Color className="size-4.5" />
          </span>
          <div className="flex min-w-0 flex-col gap-0.5">
            <DialogTitle className="font-display text-base leading-tight">
              {t("trace.install_title")}
            </DialogTitle>
            <DialogDescription className="min-h-[2.5rem] text-xs leading-snug">
              {t("trace.install_desc")}
            </DialogDescription>
          </div>
          <Button
            ref={closeBtnRef}
            variant="ghost"
            size="icon-sm"
            onClick={handleClose}
            className="ml-auto shrink-0 text-muted-foreground"
            aria-label={t("trace.install_close")}
          >
            <X className="size-4" />
          </Button>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:grid md:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)] md:overflow-hidden">
          <div className="space-y-5 border-border px-6 py-5 md:min-h-0 md:overflow-y-auto md:border-r">
            <p className="text-sm leading-relaxed text-muted-foreground">
              {t("trace.install_terminal_hint")}
            </p>
            <ol className="space-y-3">
              {[
                [Terminal, "trace.install_step_download"],
                [ShieldCheck, "trace.install_step_approve"],
              ].map(([Icon, key], index) => (
                <li key={key as string} className="flex gap-3 rounded-xl border border-border bg-secondary/35 p-3.5">
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background text-muted-foreground shadow-sm">
                    <Icon className="size-4" />
                  </span>
                  <div className="min-w-0">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/70">
                      0{index + 1}
                    </p>
                    <p className="mt-1 text-sm leading-relaxed">{t(key as string)}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>

          <div className="flex flex-col bg-[#262624] md:min-h-0 md:overflow-hidden">
            <div className="flex shrink-0 items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex min-w-0 items-center gap-2">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  {t("trace.install_script_filename")}
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                <button
                  type="button"
                  onClick={handleCopy}
                  disabled={!host}
                  className="inline-flex h-9 min-w-20 items-center justify-center gap-1.5 rounded-md px-3 text-[11px] font-medium text-white/60 transition-colors hover:bg-white/[0.08] hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40 disabled:pointer-events-none disabled:opacity-35"
                >
                  {copied ? (
                    <Check className="size-3.5 text-[#9CB071]" />
                  ) : (
                    <Copy className="size-3.5" />
                  )}
                  {copied ? t("trace.install_copied") : t("trace.install_copy")}
                </button>
              </div>
            </div>
            <div className="min-h-[280px] flex-1 overflow-auto md:min-h-0">
              <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                <code
                  className={`block font-mono whitespace-pre ${CODE_SYNTAX}`}
                  dangerouslySetInnerHTML={{ __html: highlighted }}
                />
              </pre>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: Remove `issueTraceClientTicket` from `api-client.ts`**

In `web/src/lib/api-client.ts`, delete the `issueTraceClientTicket` method and remove `IssueTraceClientTicketRsp` from the import list.

- [ ] **Step 3: Remove `IssueTraceClientTicketRsp` from `types.ts`**

In `web/src/lib/types.ts`, delete the `IssueTraceClientTicketRsp` interface.

- [ ] **Step 4: Update i18n — English**

In `web/src/locales/en.json`, update these keys:

```json
  "trace.install_desc": "Copy one command, run it in your terminal, and the script handles download, API key setup, and Codex hook configuration.",
  "trace.install_terminal_hint": "Copy and run this command. The script downloads the binary, then walks you through API key and hook setup with hidden input.",
  "trace.install_step_download": "Download the Aris binary from GitHub Releases and run a guided setup — health check, API key, and Codex hooks.",
  "trace.install_step_approve": "Review and approve the installed Aris hooks from Codex /hooks.",
```

Remove these keys: `trace.install_step_key`, `trace.install_ticket_note`, `trace.install_copying`, `trace.install_footer`.

- [ ] **Step 5: Update i18n — Chinese**

In `web/src/locales/zh.json`, update:

```json
  "trace.install_desc": "复制一条命令在终端运行，脚本自动完成下载、API Key 配置和 Codex Hook 设置。",
  "trace.install_terminal_hint": "复制并运行这段命令。脚本会下载二进制，然后通过隐藏输入引导你完成 API Key 和 Hook 配置。",
  "trace.install_step_download": "从 GitHub Releases 下载 Aris 二进制，并运行引导式配置——健康检查、API Key、Codex Hook。",
  "trace.install_step_approve": "在 Codex /hooks 中检查并手动批准新增的 Aris hooks。",
```

Remove: `trace.install_step_key`, `trace.install_ticket_note`, `trace.install_copying`, `trace.install_footer`.

- [ ] **Step 6: Update i18n — Japanese**

In `web/src/locales/ja.json`, update:

```json
  "trace.install_desc": "コマンドをコピーして端末で実行するだけで、ダウンロード、API キー設定、Codex フック設定まで自動で完了します。",
  "trace.install_terminal_hint": "このコマンドをコピーして実行します。スクリプトがバイナリをダウンロードし、非表示入力で API キーとフックの設定を案内します。",
  "trace.install_step_download": "GitHub Releases から Aris バイナリを取得し、ヘルスチェック、API キー、Codex フックのガイド付き設定を実行します。",
  "trace.install_step_approve": "Codex の /hooks で追加された Aris フックを確認して承認します。",
```

Remove: `trace.install_step_key`, `trace.install_ticket_note`, `trace.install_copying`, `trace.install_footer`.

- [ ] **Step 7: Build and verify**

Run: `cd web && npm run build`
Expected: Build succeeds without errors

- [ ] **Step 8: Commit**

```bash
git add web/
git commit -m "feat: simplify trace install dialog to curl|sh pattern

Remove ticket issuance from web UI. Install command is now a simple
curl -fsSL <host>/install.sh | sh. Steps reduced from 3 to 2."
```

---

## Task 5: CI/CD — GitHub Releases + Dockerfile

**Files:**
- Create: `.github/workflows/release-trace-client.yml`
- Modify: `docker/dockerfile` — remove cross-compile step

**Interfaces:**
- Consumes: `make build-client-all` (existing Makefile target)
- Produces: GitHub Release with 4 binary assets on tag push

- [ ] **Step 1: Create GitHub Releases workflow**

Create `.github/workflows/release-trace-client.yml`:

```yaml
name: Release Trace Client

on:
  push:
    tags: ['v*.*.*']

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.1'

      - name: Build client binaries
        run: make build-client-all

      - name: Upload to GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            build/trace-client/aris-darwin-amd64
            build/trace-client/aris-darwin-arm64
            build/trace-client/aris-linux-amd64
            build/trace-client/aris-linux-arm64
```

- [ ] **Step 2: Remove cross-compile from Dockerfile**

In `docker/dockerfile`, delete the entire "交叉编译四平台客户端" RUN block (lines 51-66) and the `COPY --from=builder /go/trace-client /app/trace-client` line (line 74).

The final stage should only have:
```dockerfile
COPY --from=builder /go/bin/aris-proxy-api /app/aris-proxy-api
```

- [ ] **Step 3: Verify Dockerfile still builds (locally)**

Run: `docker buildx build --platform linux/amd64 -t aris-proxy-api:test -f docker/dockerfile . --dry-run`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release-trace-client.yml docker/dockerfile
git commit -m "ci: publish trace client binaries to GitHub Releases

Add release-trace-client.yml workflow that cross-compiles 4 platform
binaries on tag push and uploads to GitHub Release. Remove cross-compile
step from Dockerfile — binaries no longer bundled in server image."
```

---

## Self-Review

**Spec coverage:**
- ✅ Section 2.1 Goal 1 (curl|sh command): Task 1 + Task 4
- ✅ Section 2.1 Goal 2 (install.sh self-contained): Task 1
- ✅ Section 2.1 Goal 3 (aris only trace ingest): Task 3
- ✅ Section 2.1 Goal 4 (remove ticket system): Task 2
- ✅ Section 2.1 Goal 5 (Web UI simplified): Task 4
- ✅ Section 2.1 Goal 6 (GitHub Releases): Task 5
- ✅ Section 4 (install.sh structure): Task 1 template
- ✅ Section 5 (backend changes): Task 1 + Task 2
- ✅ Section 6 (client binary changes): Task 3
- ✅ Section 7 (Web UI changes): Task 4
- ✅ Section 5.5 (CI/CD): Task 5
- ✅ Section 8 (security): API Key via stty -echo, file permissions in template
- ✅ Section 9 (testing): Task 1 unit test, Task 2 test cleanup

**Placeholder scan:** No TBD/TODO. All code blocks are complete.

**Type consistency:**
- `HandleInstallScript` signature consistent across interface (Task 1 Step 3), implementation (Task 1 Step 4), route registration (Task 1 Step 5), and test (Task 1 Step 6)
- `InstallScriptReq` consistent across DTO (Task 1 Step 2) and handler
- `TraceRouterDependencies` without `TicketStore` consistent across trace.go (Task 2 Step 1) and router.go (Task 2 Step 2)
- `TraceDependencies` without ticket/artifact/ticketStore consistent across handler.go (Task 2 Step 3) and bootstrap (Task 2 Step 7)
