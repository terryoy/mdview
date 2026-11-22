# Architecture

## Components

### Go viewer

`mdview/` is a self-contained Go HTTP service. It embeds the browser assets at build time with `go:embed`, serves the UI and static assets, and exposes authenticated local APIs for directory listings, text files, search, and image bytes.

The service resolves a root directory, keeps requests inside that root, hides dot files unless requested, and binds to loopback by default.

### MCP server

`mdview-mcp/` is a Node.js stdio MCP server. It implements the same read-oriented operations as MCP tools and serves an MCP App resource for the viewer UI. It uses `MDVIEW_ROOT` to choose the accessible root.

The MCP UI and data tools are separate concerns: a host can use the tools even when it does not render MCP App resources.

### Agent Skill

`skill/mdview/SKILL.md` teaches an agent how to discover, scope, read, and verify local files through mdview. It is deliberately not an MCP server configuration: the skill describes behavior, while the host configuration enables the server.

## Trust boundary

Both components can expose sensitive local data if the root is too broad. Use the narrowest practical root, keep the HTTP service on loopback, and review host configuration before allowing an agent to access a workspace.
