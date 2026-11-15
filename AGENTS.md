# AGENTS.md — penpot-cli

Instructions for AI coding agents. If the user asked you to install or use
`penpot-cli`, start here.

## 1. Install (macOS / Linux, no sudo)

```sh
curl -fsSL https://raw.githubusercontent.com/ans-4175/penpot-cli/main/install.sh | sh
penpot-cli version
```

- Installs to `~/.local/bin/penpot-cli` and verifies the SHA256 checksum.
- If the script prints that `~/.local/bin` is not on PATH, call the binary by its
  full path for now and tell the user the line to add to their shell profile
  (`export PATH="$HOME/.local/bin:$PATH"`). Don't edit their profile silently.
- Options: `PENPOT_CLI_VERSION=vX.Y.Z`, `INSTALL_DIR=/other/dir`.
- **Windows:** download `penpot-cli-windows-<amd64|arm64>.zip` from
  https://github.com/ans-4175/penpot-cli/releases/latest, unzip, put
  `penpot-cli.exe` on PATH.
- macOS Gatekeeper ("developer cannot be verified") only affects files downloaded
  through a browser. If it happens: `xattr -d com.apple.quarantine <path>/penpot-cli`.
- Don't build from source just to install; the release binary is the supported path.

## 2. Configure (the user does the secret part)

The user needs a Penpot MCP key (Penpot → Your account → Integrations → MCP Server).

- **Never** ask the user to paste the key into chat, print it, or pass it as a
  command argument. Have them write it to `~/.config/ans/penpot` (mode 600):
  ```
  PENPOT_DOMAIN=design.penpot.app
  PENPOT_MCP_TOKEN=<key>
  ```
- Optional `PENPOT_PERSONAL_TOKEN=<token>` unlocks `backup`, `sync`, `library`, `app --file`.
- The user must open Penpot (`penpot-cli app --file <id>`; on Linux/Windows this is untested, fall back to a normal visible Chrome tab) and click
  **File → MCP Server → Connect**. You cannot do this for them.

## 3. Verify and learn

```sh
penpot-cli doctor      # daemon → MCP session → plugin; shows focused file/page
penpot-cli --skills    # the full agent playbook, embedded in the binary
```

Read `penpot-cli --skills` before touching the canvas. Then `penpot-cli state` and
**confirm the target file/page with the user** before any write. Run
`penpot-cli backup save "<label>"` before big changes. Never bulk-delete shapes.

## Developing this repo

Go ≥ 1.23, stdlib only. `gofmt -l . && go vet ./... && go test ./...`.
Architecture and measured limits: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) (Indonesian).
Releases: push a `vX.Y.Z` tag; `.github/workflows/release.yml` builds and publishes.
