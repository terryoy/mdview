---
name: mdview
description: Browse, search, read, and visually inspect local workspace files through mdview or mdview-mcp
---

# mdview Skill

Use this skill when a task requires inspecting local Markdown reports, source files, images, or a workspace directory through mdview.

## Choose an access path

1. Prefer the `mdview-mcp` tools when they are available in the current agent session.
2. Use the browser viewer when a human needs visual Markdown rendering, an outline, themes, or interactive browsing.
3. If neither is available, clearly state the limitation instead of pretending that a file was inspected.

## Safe workflow

1. Identify the configured root before reading files. Do not assume that a relative path is rooted at the repository or Home.
2. Start with `list_dir` for the smallest relevant directory.
3. Use `search_files` to locate reports or symbols before reading large files.
4. Read only the files needed for the task with `read_file`.
5. Use `read_blob` only for supported images and only when visual inspection is needed.
6. Treat local files as potentially sensitive. Do not paste secrets, tokens, private keys, or unrelated personal content into the conversation.
7. When reporting a result, include the relative path and distinguish observed content from an inference.

## Tool guidance

- `list_dir(path, all=false)`: inspect a directory; keep hidden files excluded unless the user explicitly needs them.
- `read_file(path)`: read UTF-8 text; use it for Markdown, code, JSON, YAML, and logs.
- `search_files(query, path)`: search names and text; narrow `path` before broad searches.
- `read_blob(path)`: inspect a PNG/JPEG/GIF/SVG/WebP image.
- `show_viewer(path?)`: open the visual viewer only when the host supports MCP Apps.
- `show_demo()`: diagnose MCP App rendering and tool-bridge support; a timeout means the host UI bridge is unavailable, not that the data tools are broken.

## Path and scope rules

- Never use `..` to escape the configured root.
- Prefer explicit paths supplied by the user.
- Avoid recursive Home-wide searches unless necessary; they can be slow and reveal unrelated files.
- Do not modify or delete files merely to inspect them.
- Before any write operation, confirm the exact target and preserve existing user changes.

## Verification checklist

Before claiming that a file or report was found:

- [ ] The path was returned by `list_dir` or `search_files`, or explicitly supplied by the user.
- [ ] The relevant content was read successfully.
- [ ] Any image claim was based on `read_blob`/visual inspection.
- [ ] UI claims distinguish viewer rendering from MCP data-tool availability.
- [ ] No sensitive unrelated content was disclosed.
