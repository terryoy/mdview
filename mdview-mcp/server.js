#!/usr/bin/env node
/**
 * mdview-mcp — 本地文件桥 MCP Server + MCP App UI
 * 工具: show_viewer / list_dir / read_file / search_files / read_blob
 * 资源: ui://mdview/viewer (text/html;profile=mcp-app)
 * 根目录: MDVIEW_ROOT 环境变量，默认 Home
 */
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  ListResourcesRequestSchema,
  ReadResourceRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, dirname, extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { homedir } from "node:os";

const VERSION = "2.0.0";
const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(process.env.MDVIEW_ROOT || homedir());
const VIEWER_URI = "ui://mdview/viewer";
const DEMO_URI = "ui://mdview/demo";

/* 极简桥接自检应用（3KB）：渲染成功即证明通道，内嵌①②两步桥接测试 */
const DEMO_HTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><style>
body{font-family:system-ui,"Noto Sans CJK SC",sans-serif;padding:20px;line-height:1.9;background:#f6f8fa;color:#1f2328}
.box{background:#fff;border:1px solid #d0d7de;border-radius:8px;padding:16px;max-width:640px}
</style></head><body><div class="box">
<h3>🧪 mdview MCP 桥接自检</h3>
<div id="log">启动…</div>
</div>
<script>
const log = m => document.getElementById('log').innerHTML += '<br>' + m;
let rid = 0; const pending = new Map();
window.addEventListener('message', e => {
  const d = e.data;
  if (d && d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); }
});
function req(method, params) {
  return new Promise(res => { const i = ++rid; pending.set(i, res);
    window.parent.postMessage({jsonrpc:'2.0',id:i,method,params}, '*');
    setTimeout(() => { if (pending.has(i)) { pending.delete(i); res({timeout:true}); } }, 8000);
  });
}
(async () => {
  log('✅ 应用已渲染（ui:// 资源通路正常）');
  log('① 测试 ui/initialize …');
  const init = await req('ui/initialize', {});
  log(init.timeout ? '① ❌ 超时：宿主未响应（桥不可用）' : '① 响应：' + JSON.stringify(init.result || init.error || {}).slice(0,160));
  log('② 测试 tools/call list_dir …');
  const call = await req('tools/call', {name:'list_dir', arguments:{path:'agent-workstation'}});
  if (call.timeout) log('② ❌ 超时：应用→MCP 桥不通');
  else if (call.error) log('② ❌ ' + JSON.stringify(call.error).slice(0,200));
  else log('② ✅ 工具返回：' + String((call.result&&call.result.content&&call.result.content[0].text)||'').slice(0,300));
  log('—— 自检结束 ——');
})();
</script></body></html>`;

/* ── 静态资源内联（应用零网络依赖） ── */
function buildHtml() {
  const dir = join(__dirname, "assets");
  const read = (f) => readFileSync(join(dir, f), "utf-8");
  let html = read("index.html");
  html = html.replace(/<!--INLINE_JS:(\S+)-->/g, (_, f) =>
    "<script>\n" + read(f).replace(/<\/script>/gi, "<\\/script>") + "\n</script>");
  html = html.replace(/<!--INLINE_CSS:([\w-]+):(\S+)-->/g, (_, id, f) =>
    `<style id="${id}">\n${read(f)}\n</style>`);
  return html;
}
let APP_HTML = "";
try { APP_HTML = buildHtml(); } catch (e) { console.error("HTML 构建失败:", e.message); }

/* ── 路径安全 ── */
function safePath(rel = "") {
  const p = join(ROOT, resolve("/", String(rel)));
  if (p !== ROOT && !p.startsWith(ROOT + sep)) throw new Error("forbidden: 越界访问");
  return p;
}
const relOf = (abs) => (abs === ROOT ? "" : abs.slice(ROOT.length + 1).split(sep).join("/"));

/* ── 工具实现 ── */
const TEXT_EXTS = new Set(["md","markdown","txt","go","py","js","ts","jsx","tsx","vue","json","yml","yaml","toml","sh","bash","html","css","rs","c","h","cpp","hpp","java","xml","ini","conf","cfg","log","sql","lua","rb"]);
const SKIP_DIRS = new Set(["node_modules","vendor","__pycache__","target","dist","build",".git",".cache"]);
const IMG_MIME = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", svg: "image/svg+xml", webp: "image/webp" };

function listDir(rel = "", all = false) {
  const abs = safePath(rel);
  const out = [];
  for (const name of readdirSync(abs)) {
    if (!all && name.startsWith(".")) continue;
    const p = join(abs, name);
    let st;
    try { st = statSync(p); } catch { continue; }
    const isDir = st.isDirectory();
    out.push({
      name, path: relOf(p), isDir,
      size: isDir ? 0 : st.size,
      modTime: Math.floor(st.mtimeMs / 1000),
      ext: isDir ? "" : extname(name).slice(1).toLowerCase(),
    });
  }
  out.sort((a, b) => (a.isDir === b.isDir
    ? a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    : a.isDir ? -1 : 1));
  return out;
}

function readText(rel) {
  const abs = safePath(rel);
  const st = statSync(abs);
  if (st.isDirectory()) throw new Error("该路径是目录");
  if (st.size > 8 * 1024 * 1024) throw new Error("文件过大(>8MB)");
  return readFileSync(abs, "utf-8");
}

function readBlob(rel) {
  const abs = safePath(rel);
  const ext = extname(abs).slice(1).toLowerCase();
  const mime = IMG_MIME[ext];
  if (!mime) throw new Error("不支持的图片类型: " + ext);
  const st = statSync(abs);
  if (st.size > 8 * 1024 * 1024) throw new Error("图片过大(>8MB)");
  return { mimeType: mime, base64: readFileSync(abs).toString("base64") };
}

function searchFiles(query, scope = "", all = false) {
  const q = String(query || "").toLowerCase().trim();
  const results = [];
  if (!q) return results;
  const base = safePath(scope);
  const MAX = 200;
  let visited = 0;
  const walk = (dir) => {
    if (results.length >= MAX || visited > 200000) return;
    let names;
    try { names = readdirSync(dir); } catch { return; }
    for (const name of names) {
      if (results.length >= MAX) return;
      if (!all && name.startsWith(".")) continue;
      const p = join(dir, name);
      let st;
      try { st = statSync(p); } catch { continue; }
      visited++;
      if (st.isDirectory()) { if (!SKIP_DIRS.has(name)) walk(p); continue; }
      const rel = relOf(p);
      if (name.toLowerCase().includes(q)) results.push({ path: rel, line: 0, text: "", name: true });
      const ext = extname(name).slice(1).toLowerCase();
      if (!TEXT_EXTS.has(ext) || st.size > 2 * 1024 * 1024) continue;
      let content;
      try { content = readFileSync(p, "utf-8"); } catch { continue; }
      let matches = 0;
      const lines = content.split("\n");
      for (let i = 0; i < lines.length && matches < 3 && results.length < MAX; i++) {
        if (lines[i].toLowerCase().includes(q)) {
          let t = lines[i].trim();
          if (t.length > 160) t = t.slice(0, 160) + "…";
          results.push({ path: rel, line: i + 1, text: t, name: false });
          matches++;
        }
      }
    }
  };
  walk(base);
  return results;
}

/* ── MCP Server ── */
const server = new Server(
  { name: "mdview", version: VERSION },
  { capabilities: { tools: {}, resources: {} } }
);
const asText = (v) => [{ type: "text", text: typeof v === "string" ? v : JSON.stringify(v) }];

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [
    {
      name: "show_viewer",
      description: "打开 mdview 文件浏览器界面（MCP App）。可选 path 指定要打开的文件（相对 Home）。",
      inputSchema: { type: "object", properties: { path: { type: "string", description: "相对 Home 的文件路径，可选" } } },
    },
    {
      name: "show_demo",
      description: "打开 mdview 桥接自检 demo（极简 MCP App，用于诊断渲染与桥接通路）",
      inputSchema: { type: "object", properties: {} },
    },
    {
      name: "list_dir",
      description: "列出目录内容（默认隐藏 . 开头文件；path 相对 Home，空串=根目录）",
      inputSchema: { type: "object", properties: { path: { type: "string" }, all: { type: "boolean", description: "显示隐藏文件" } } },
    },
    {
      name: "read_file",
      description: "读取文本文件内容（UTF-8，≤8MB）",
      inputSchema: { type: "object", properties: { path: { type: "string" } }, required: ["path"] },
    },
    {
      name: "search_files",
      description: "搜索文件名与文本内容（默认范围 Home；跳过隐藏文件与 node_modules 等）",
      inputSchema: { type: "object", properties: { query: { type: "string" }, path: { type: "string", description: "搜索范围子目录，可选" } }, required: ["query"] },
    },
    {
      name: "read_blob",
      description: "读取图片为 base64（png/jpg/jpeg/gif/svg/webp）",
      inputSchema: { type: "object", properties: { path: { type: "string" } }, required: ["path"] },
    },
  ],
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args = {} } = request.params;
  try {
    switch (name) {
      case "show_viewer":
        return { content: asText(`mdview 界面已打开${args.path ? "：" + args.path : ""}`), _meta: { ui: { resourceUri: VIEWER_URI } } };
      case "show_demo":
        return { content: asText("mdview 桥接自检 demo 已打开"), _meta: { ui: { resourceUri: DEMO_URI } } };
      case "list_dir":
        return { content: asText(listDir(args.path || "", !!args.all)) };
      case "read_file":
        return { content: asText(readText(args.path)) };
      case "search_files":
        return { content: asText(searchFiles(args.query, args.path || "", !!args.all)) };
      case "read_blob":
        return { content: asText(readBlob(args.path)) };
      default:
        throw new Error("Unknown tool: " + name);
    }
  } catch (e) {
    return { content: [{ type: "text", text: "❌ " + e.message }], isError: true };
  }
});

server.setRequestHandler(ListResourcesRequestSchema, async () => ({
  resources: [{
    uri: VIEWER_URI,
    name: "mdview 文件预览器",
    description: "本地 Markdown/文件浏览界面",
    mimeType: "text/html;profile=mcp-app",
  }, {
    uri: DEMO_URI,
    name: "mdview 桥接自检 demo",
    description: "极简 MCP App，用于诊断渲染与桥接通路",
    mimeType: "text/html;profile=mcp-app",
  }],
}));

server.setRequestHandler(ReadResourceRequestSchema, async (request) => {
  const mk = (uri, html) => ({
    contents: [{
      uri,
      mimeType: "text/html;profile=mcp-app",
      text: html,
      _meta: { ui: { csp: { connectDomains: [], resourceDomains: [], frameDomains: [], baseUriDomains: [] }, prefersBorder: true } },
    }],
  });
  if (request.params.uri === VIEWER_URI) return mk(VIEWER_URI, APP_HTML);
  if (request.params.uri === DEMO_URI) return mk(DEMO_URI, DEMO_HTML);
  throw new Error("Resource not found: " + request.params.uri);
});

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error(`mdview-mcp v${VERSION} running on stdio (root: ${ROOT})`);
}
main().catch((e) => { console.error(e); process.exit(1); });
