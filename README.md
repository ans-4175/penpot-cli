# penpot-cli

**Your coding agent can't touch Penpot.** You say "make me a landing page in Penpot", it opens a text editor, writes some CSS, and apologises. Penpot has a canvas. Your agent has a terminal. Nobody is introducing them.

`penpot-cli` is the introduction. One small program that lets your AI agent (Claude Code, Codex, Cursor, Hermes, whatever lives in your terminal) *work inside your open Penpot file*: read the boards, the layers, the colors, the components, then build, fix and export things on the real canvas. Live, in your browser, while you watch.

> Designer: "Make it look like this screenshot, but in Penpot."
> Agent, before: "Here's a `<div>` with a gradient. Hope that helps."
> Agent, after: "Hero board 1440×900, flex column, Inter 56/60, CTA `#2563EB`, three feature cards as components. Done. Here's the export, check the spacing."

Zero dependencies, one binary, no installer.

---

## What can I use it for?

- **Screenshot → editable design.** Drop in a reference image (a competitor, a Dribbble shot, last year's mockup). The agent pins it next to your work, then rebuilds it as real boards, auto-layout and text, not a flattened picture.
- **Landing pages, ads, carousels, slides.** Describe the thing and the agent builds it section by section, exporting each one so it can look at its own work and fix it. There's a built-in recipe for LinkedIn carousels, story ads, and landing sections.
- **Code → design.** Point it at a React/Tailwind project. It scans your real components and recreates them on the canvas, so the Penpot file matches what's actually shipped.
- **Design → code.** Select a board, run `codegen bundle`, and your agent gets structure, spacing, colors and fonts as data. No more "eyeballing it from a screenshot".
- **"Explain this file to me."** Inherited a 40-board Penpot file nobody documented? `state` gives the agent the pages, boards, selection and libraries, so it can give you a proper tour.
- **Design-system housekeeping.** List and create components, export design tokens (W3C format), publish or link a Shared Library.
- **Visual QA.** Export any board as PNG/SVG and diff it against the reference (`Reference | Penpot | Difference` side by side).
- **Safety net.** One command snapshots the file (`backup save`) before the agent does anything big, and another brings it back.
- **Move off pen.dev.** A porter script converts `.pen` files into Penpot boards.
- **Anything else Penpot's Plugin API can do.** `penpot-cli exec` runs JavaScript inside your open file. If the API can do it, your agent can do it.

---

## Install

### 🤖 Quick prompt: let your agent install it

Paste this into Claude Code, Codex, Cursor, or whatever agent you use:

```text
Install penpot-cli from https://github.com/ans-4175/penpot-cli on this machine.

- macOS/Linux: run
  curl -fsSL https://raw.githubusercontent.com/ans-4175/penpot-cli/main/install.sh | sh
  It needs no sudo. Never use sudo or ask for my password.
- Windows: download penpot-cli-windows-<amd64|arm64>.zip from
  https://github.com/ans-4175/penpot-cli/releases/latest, unzip it, and put
  penpot-cli.exe in a folder on my PATH.
- Check it worked with `penpot-cli version`. If ~/.local/bin isn't on my PATH, show me
  the exact line to add to my shell profile and ask before editing it.
- Then read https://github.com/ans-4175/penpot-cli/blob/main/AGENTS.md and tell me
  what I still have to do myself (the Penpot key and connecting the plugin).
  Never ask me to paste my key into chat.
```

### Fastest: one line (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ans-4175/penpot-cli/main/install.sh | sh
penpot-cli version
```

It detects your Mac/Linux and CPU, downloads the matching [release](../../releases/latest), **verifies the checksum**, and installs to `~/.local/bin`. No `sudo`, no password prompt, so an AI agent can run it for you. If it says `~/.local/bin` isn't on your PATH, add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile.

Pin a version with `PENPOT_CLI_VERSION=v2.6.2`, or pick another folder with `INSTALL_DIR=/some/dir`.

### Manual download

Grab the file for your computer from [Releases](../../releases).

| You have | Download |
|---|---|
| Mac (M1/M2/M3/M4) | `penpot-cli-darwin-arm64.tar.gz` |
| Mac (Intel) | `penpot-cli-darwin-amd64.tar.gz` |
| Linux | `penpot-cli-linux-amd64.tar.gz` (or `-arm64`) |
| Windows | `penpot-cli-windows-amd64.zip` (or `-arm64`) |

```bash
tar -xzf penpot-cli-<os>-<arch>.tar.gz
sudo mv penpot-cli /usr/local/bin/
penpot-cli version        # prints a version = you're in
```

> 🍎 Downloaded in a **browser** and macOS says "cannot be opened because the developer cannot be verified"? The binary isn't notarized. Run `xattr -d com.apple.quarantine <path-to>/penpot-cli` once. Files fetched with `curl` or the one-liner above never get that flag.

**Windows**: unzip, put `penpot-cli.exe` in a folder on your PATH.

> 🧪 Honest footnote: the author drives this daily on macOS. Linux and Windows builds are compiled and unit-tested in CI on every push, but **not yet tested by a human**, especially `penpot-cli app` on Windows (it hasn't been run on a real Windows machine). If you try it, tell us how it went. One feature is macOS-only: `upload --max` (image resize, uses `sips`). Everything else is the same everywhere.

---

## Setup: one key, one open tab

### 1. Get your Penpot MCP key

In Penpot: avatar → **Your account → Integrations → MCP Server** → enable it and copy the key. Treat it like a password.

### 2. Put it in a private file

Not in chat, not in the repo.

```bash
mkdir -p ~/.config/ans && chmod 700 ~/.config/ans
printf 'Paste your key, then Enter: '; read -rs T; echo
printf 'PENPOT_DOMAIN=design.penpot.app\nPENPOT_MCP_TOKEN=%s\n' "$T" > ~/.config/ans/penpot
unset T; chmod 600 ~/.config/ans/penpot
```

Self-hosted Penpot? Change `PENPOT_DOMAIN`, or set `PENPOT_MCP_URL` to the full MCP URL. Windows: same file at `%USERPROFILE%\.config\ans\penpot`.

*Optional:* add `PENPOT_PERSONAL_TOKEN=<token>` (Your account → Access tokens) to unlock `backup`, `sync`, `library` and `app --file`.

### 3. Open Penpot and connect the plugin

Your agent works through the Penpot **tab you have open**. That tab is its hands, so it must stay alive.

- **Recommended:** `penpot-cli app --file <fileId>` opens a dedicated Chrome window that browsers can't put to sleep, even when you hide it. Log in once. It finds Chrome/Chromium for you (on Linux/Windows pass `--chrome <path>` if it can't). *Tested on macOS; not yet on Linux/Windows.*
- **Fallback:** open Penpot in Chrome yourself and keep that tab visible (browsers throttle hidden tabs, and the plugin goes quiet).

Then, in Penpot: **File → MCP Server → Connect**.

### 4. Check it

```bash
penpot-cli doctor
```

`doctor` checks the daemon, the MCP session and the plugin, and tells you which file and page the agent is looking at. All green = go.

---

## 🤖 Let your agent set it up

**Not installed yet?** Paste this into your agent:

```text
Install penpot-cli on this machine: run
`curl -fsSL https://raw.githubusercontent.com/ans-4175/penpot-cli/main/install.sh | sh`
(no sudo needed), then confirm with `penpot-cli version`. If it says ~/.local/bin is
not on PATH, tell me the line to add to my shell profile instead of editing it silently.
Then read https://github.com/ans-4175/penpot-cli/blob/main/AGENTS.md for next steps.
```

**Already installed?** Paste this instead:

```text
Help me set up penpot-cli. First run `penpot-cli --skills` and follow it.
Rules: never ask me to paste my Penpot key into chat, never print it, never put
it in a command argument. Guide me to save it in ~/.config/ans/penpot (mode 600),
then run `penpot-cli doctor` and tell me the result.
```

**Teach your agent to use it** (put this in `CLAUDE.md` / `AGENTS.md`):

```text
For anything on a Penpot canvas, use penpot-cli. Run `penpot-cli --skills` once to
learn it. Start every task with `penpot-cli state` and confirm the file/page with me.
Never delete shapes in bulk. Run `penpot-cli backup save "<label>"` before big changes.
```

The agent playbook lives inside the binary (`penpot-cli --skills`), so it always matches your version.

---

## 🚀 First try

Open any file in Penpot (with the plugin connected), then:

```bash
penpot-cli state
```

That's the readable outline your agent sees: pages, boards, what you've selected, the libraries. Now hand it over:

```text
Use penpot-cli. Look at the board I've selected and describe its layout,
colors and type. Then build a mobile version next to it. Ask me before
touching any other page.
```

More things worth trying on day one:

```text
Here's a screenshot: ~/Downloads/ref.png. Import it as a reference board, then
rebuild it as a real Penpot design using auto-layout. Export each section and
compare it to the reference before moving on.
```
```text
Make a 5-slide LinkedIn carousel (1080×1350) about <topic> on a new page.
Keep the style consistent, and export every slide so we can review them.
```
```text
Scan ./src (React + Tailwind) and recreate the Button, Card and Navbar as
components on a "Components" page.
```
```text
Select my pricing board and generate a design-to-code bundle. Then build it
in this repo.
```

---

## The cheat sheet

You mostly talk to your agent. But the commands are there when you want them:

```bash
penpot-cli state                          # what's open: pages, boards, selection, libraries
penpot-cli exec -f build-hero.js          # run JavaScript on the canvas
penpot-cli export selection --out hero.png   # render for review
penpot-cli upload ./assets --max 1400     # push images into the file (macOS: with resize)
penpot-cli reference import shot.png      # pin a screenshot as a locked reference board
penpot-cli component create <id> --name 'Button / Primary'
penpot-cli tokens export --out tokens.json   # design tokens (W3C format)
penpot-cli backup save "before hero redo" # snapshot; `backup list` / `backup restore <id>`
penpot-cli sync                           # prove your changes reached the server
penpot-cli codegen bundle <id> --out ./bundle   # board → design-to-code evidence
penpot-cli pin                            # lock the target file; refuse to write if you switch
```

Everything has `--help`. Full list: `penpot-cli help`.

---

## When things go sideways

| You see | Probably means |
|---|---|
| `doctor` says plugin not connected | Penpot tab isn't open, or you haven't done **File → MCP Server → Connect**. |
| Commands hang, then work after a minute | The tab fell asleep. The CLI waits up to 3 minutes for it. Use `penpot-cli app` to prevent this. |
| `403` | Wrong or revoked key (or a proxy blocking the request). Re-check `~/.config/ans/penpot`. |
| `413` | Request too big. `exec` splits big scripts itself; for images use `upload`, not inline base64. |
| "Cannot modify a page that is not currently active" | The agent tried to edit a page you're not on. Tell it to open the page first. |
| Changes not showing up on the server | Run `penpot-cli sync` and check the revision went up. |

There are hard limits on Penpot's side (≈100 KB per request, 15 MB per response, 120 s per run). penpot-cli works around most of them. Details in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

---

## For the curious (and the contributors)

- How it works: a tiny background daemon keeps **one** MCP session to Penpot alive, so each command takes ~0.3 s instead of ~1.5 s. It starts itself and shuts down after 15 idle minutes.
- Architecture, measured limits, code tour: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) *(in Indonesian)*
- The agent playbook: `penpot-cli --skills`
- Build it yourself (Go ≥ 1.23, stdlib only):

  ```bash
  go build -o penpot-cli .
  ```

Runs on macOS, Linux and Windows, Intel and ARM. One file, nothing else to install.

## License

[MIT](LICENSE) © Anshorimuslim ([@ans-4175](https://github.com/ans-4175)). Use it, fork it, change it, ship it. Just keep the copyright notice and license text with any copy or derivative, which is how you credit the original.
