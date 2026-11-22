# mdview

> A local-first Markdown/file bridge and browser viewer for humans and AI agents.

mdview turns a local workspace into a searchable, readable, agent-accessible surface without uploading files to a cloud service. It provides:

- **Go viewer service**: a single binary with directory browsing, Markdown rendering, syntax highlighting, search, outline navigation, themes, and resizable panels.
- **MCP server**: stdio tools for agents—`list_dir`, `read_file`, `search_files`, `read_blob`, `show_viewer`, and `show_demo`.
- **Agent Skill**: reusable instructions for safely using mdview from goose or another Agent Skills-compatible agent.

## Why it is useful

Personal workstations contain reports, notes, code, screenshots, and project artifacts. Agents need controlled access to those artifacts, while people need a fast visual way to inspect the same files. mdview keeps both paths local:

```text
Human browser ──HTTP──> mdview ──> local files
Agent ──MCP/stdio──> mdview-mcp ──> local files
```

Typical uses:

- Read and search a large report directory without opening an editor.
- Give an agent read-only access to a selected workspace.
- Browse generated Markdown reports with a table of contents and dark mode.
- Inspect images and source files during debugging.
- Build a lightweight local knowledge surface for an Agent-centric workstation.

## Requirements

- Linux/macOS/Windows for the Go binary (tested primarily on Linux)
- Go 1.26+ to build the viewer
- Node.js 20+ and npm for the MCP server
- A modern browser
- An MCP-compatible host for the MCP integration

## Quick start: browser viewer

```bash
git clone https://github.com/terryoy/mdview.git
cd mdview/mdview
go build -o mdview .
./mdview --root "$HOME/Documents"
```

Open <http://127.0.0.1:8377>. The service defaults to the user's Home directory when `--root` is omitted.

Useful options:

```bash
./mdview                         # browse Home
./mdview ./reports               # browse a specific directory
./mdview --addr 127.0.0.1:9000  # use another address/port
./mdview --no-open               # serve without launching a browser
./mdview -a                      # include dot files
```

The default listener is loopback-only. The web UI supports directory trees, Markdown/GFM, code highlighting, full-text search, images, outlines, themes, wide/narrow reading modes, and draggable left/right panels. Panel widths are remembered in the browser; double-click a divider to reset it.

## MCP server

The MCP server uses stdio and defaults to the user's Home directory:

```bash
cd mdview-mcp
npm ci
MDVIEW_ROOT="$HOME/agent-workstation" node server.js
```

Configure the MCP host with the **absolute path** to `server.js`; this avoids PATH wrappers changing the process and breaking stdio handshakes. A generic stdio configuration is:

```json
{
  "type": "stdio",
  "name": "mdview",
  "cmd": "/absolute/path/to/node",
  "args": ["/absolute/path/to/mdview/mdview-mcp/server.js"],
  "envs": {
    "MDVIEW_ROOT": "/home/you/workspace"
  }
}
```

The exact configuration key names depend on the MCP host. The server exposes:

| Tool | Purpose |
|---|---|
| `list_dir` | List a directory, hiding dot files by default |
| `read_file` | Read UTF-8 text up to 8 MiB |
| `search_files` | Search file names and supported text files |
| `read_blob` | Read supported images as base64 |
| `show_viewer` | Open the MCP App viewer when the host supports MCP Apps |
| `show_demo` | Run the UI/bridge diagnostic when the host supports MCP Apps |

`show_viewer` and `show_demo` require an MCP host with MCP Apps UI rendering. The data tools work independently of UI rendering.

## Agent Skill

The repository includes `skill/mdview/SKILL.md`. Install it as a project skill:

```bash
mkdir -p .agents/skills
cp -R skill/mdview .agents/skills/
```

Or install globally:

```bash
mkdir -p ~/.agents/skills
cp -R skill/mdview ~/.agents/skills/
```

Then ask the agent to use the `mdview` skill when browsing or searching local workspace files. The skill is intentionally read-oriented and includes boundary checks, path handling, and verification steps.

## Security model

- The Go service binds to `127.0.0.1` by default.
- The MCP server accesses the directory in `MDVIEW_ROOT` (Home by default).
- Paths are checked to prevent traversal outside the configured root.
- Dot files are hidden by default in directory listings/search.
- Do not expose the service on a non-loopback address without adding authentication and network controls appropriate for your environment.
- Do not commit tokens, private reports, generated snapshots, or personal configuration.

## Development

```bash
cd mdview
go test ./...
go build -o /tmp/mdview .

cd ../mdview-mcp
npm ci
node --check server.js
```

The browser assets are embedded into the Go binary. After editing `mdview/assets`, rebuild the binary. The MCP App assets are embedded by `server.js` at startup.

## Repository layout

```text
mdview/
├── mdview/                 # Go HTTP service and embedded browser UI
├── mdview-mcp/             # Node stdio MCP server and MCP App UI
├── skill/mdview/SKILL.md   # Agent Skill
├── docs/                   # Guides and examples
├── examples/               # Example deployment/configuration files
├── LICENSE                 # Apache-2.0
└── README.md
```

## License

Apache-2.0. See [LICENSE](LICENSE).
