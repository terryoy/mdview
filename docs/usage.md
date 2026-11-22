# Usage guide

## Scenario 1: read a report directory

```bash
./mdview --root "$HOME/agent-workstation/reports"
```

Open the local URL in a browser. Expand folders, select a Markdown file, use the outline on the right, and drag either divider to change the layout. Use the search box to find report names or content.

## Scenario 2: give an agent a bounded workspace

```bash
MDVIEW_ROOT="$HOME/agent-workstation" \
  node mdview-mcp/server.js
```

The agent can first list `reports`, search for a topic, and read only the matching files. Set `MDVIEW_ROOT` to a project directory instead of Home when broad access is unnecessary.

Example interaction:

```text
List the reports directory, find documents mentioning "MCP Apps", and summarize only those documents.
```

A safe agent should discover paths before reading, preserve relative paths in citations, and avoid exposing unrelated private files.

## Scenario 3: inspect an image referenced by a report

Use `read_blob` for a supported local image. The tool returns its MIME type and base64 payload; the agent or host can render it for inspection.

## Scenario 4: troubleshoot MCP App support

Call `show_demo` when the MCP host exposes UI resources. A successful render and the two bridge checks indicate UI and tool-call connectivity. If the UI times out, continue using `list_dir`, `read_file`, `search_files`, and `read_blob`; MCP Apps UI support is host-dependent.

## Configuration notes

- Keep the Go HTTP service on loopback unless you have a deliberate network security design.
- Use absolute executable paths in stdio host configuration when PATH wrappers are known to interfere with handshakes.
- The viewer token/authentication behavior is implementation-specific; inspect the source and local deployment configuration before exposing it beyond localhost.
