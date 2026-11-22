// mdview — 本地 Markdown 文件桥 + 预览器（工作站 1 号工具）
// v1.1: token 鉴权 / PNA 跨域 / 隐藏文件过滤 / 全文搜索 / systemd 常驻
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed assets
var assets embed.FS

const tokenPlaceholder = "__MDVIEW_TOKEN__"
const version = "1.1.0"

var (
	root       string
	token      string
	showHidden bool
)

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	Ext     string `json:"ext"`
}

type SearchResult struct {
	Path string `json:"path"` // 相对 root
	Line int    `json:"line"` // 内容匹配行号（文件名匹配为0）
	Text string `json:"text"` // 匹配行内容摘要
	Name bool   `json:"name"` // true=文件名匹配
}

func main() {
	home, _ := os.UserHomeDir()
	var addr, rootFlag string
	var noOpen, all bool
	flag.StringVar(&rootFlag, "root", "", "根目录（默认 Home）")
	flag.StringVar(&addr, "addr", "127.0.0.1:8377", "监听地址")
	flag.BoolVar(&noOpen, "no-open", false, "不自动打开浏览器（服务模式）")
	flag.BoolVar(&all, "a", false, "显示隐藏文件（. 开头）")
	flag.Parse()
	showHidden = all

	target := rootFlag
	openPath := ""
	if target == "" && flag.NArg() > 0 {
		target = flag.Arg(0)
	}
	switch {
	case target != "":
		abs, err := filepath.Abs(target)
		if err != nil {
			log.Fatal(err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			log.Fatalf("❌ 路径不存在: %s", abs)
		}
		if info.IsDir() {
			root = abs
		} else {
			root = filepath.Dir(abs)
			openPath = info.Name()
		}
	default:
		root = home
	}

	token = loadOrCreateToken()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	sub, _ := fs.Sub(assets, "assets")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/api/health", handleHealth) // 探活免鉴权
	mux.HandleFunc("/api/tree", auth(handleTree))
	mux.HandleFunc("/api/file", auth(handleFile))
	mux.HandleFunc("/api/raw", auth(handleRaw))
	mux.HandleFunc("/api/search", auth(handleSearch))

	// CORS + PNA 包装：允许 Goose 应用等本地页面跨域调用（仅 GET，仅本机回环）
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Header.Get("Access-Control-Request-Private-Network") != "" {
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("❌ 地址被占用 %s: %v（可用 -addr 换端口）", addr, err)
	}
	url := "http://" + ln.Addr().String() + "/"
	if openPath != "" {
		url += "?p=" + openPath
	}
	fmt.Printf("📖 mdview v%s 运行中: %s\n📂 根目录: %s\n🔑 访问令牌: %s（存于 ~/.config/mdview/token）\n⏹  Ctrl+C 退出\n", version, url, root, token)
	if !noOpen {
		go exec.Command("xdg-open", url).Start()
	}
	log.Fatal(http.Serve(ln, handler))
}

// ── 鉴权 ──

func loadOrCreateToken() string {
	if t := os.Getenv("MDVIEW_TOKEN"); t != "" {
		return t
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = "."
	}
	tfile := filepath.Join(cfgDir, "mdview", "token")
	if data, err := os.ReadFile(tfile); err == nil {
		if t := strings.TrimSpace(string(data)); t != "" {
			return t
		}
	}
	os.MkdirAll(filepath.Dir(tfile), 0700)
	b := make([]byte, 16)
	rand.Read(b)
	t := hex.EncodeToString(b)
	if err := os.WriteFile(tfile, []byte(t+"\n"), 0600); err != nil {
		log.Printf("⚠️ 无法保存 token: %v", err)
	}
	return t
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tok == "" {
			tok = r.URL.Query().Get("token")
		}
		if tok != token {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next(w, r)
	}
}

// ── 页面与静态资源 ──

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "index.html missing", 500)
		return
	}
	// 注入 token：同源页面免手动配置
	html := strings.ReplaceAll(string(data), tokenPlaceholder, token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "version": version})
}

// ── 路径安全 ──

func safePath(rel string) (string, error) {
	clean := filepath.Clean(string(filepath.Separator) + rel)
	p := filepath.Join(root, clean)
	if p != root && !strings.HasPrefix(p, root+string(filepath.Separator)) {
		return "", fmt.Errorf("forbidden")
	}
	return p, nil
}

func joinRel(parent, name string) string {
	parent = strings.Trim(parent, "/")
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

// ── 目录列表（默认隐藏 . 开头文件） ──

func handleTree(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(r.URL.Query().Get("path"), "/")
	all := showHidden || r.URL.Query().Get("all") == "1"
	p, err := safePath(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	des, err := os.ReadDir(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list := make([]Entry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if !all && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := Entry{
			Name:    name,
			Path:    joinRel(rel, name),
			IsDir:   de.IsDir(),
			ModTime: info.ModTime().Unix(),
		}
		if !de.IsDir() {
			e.Size = info.Size()
			e.Ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		}
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDir != list[j].IsDir {
			return list[i].IsDir
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	writeJSON(w, list)
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	p, err := safePath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info.Size() > 8<<20 {
		http.Error(w, "文件过大(>8MB)", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func handleRaw(w http.ResponseWriter, r *http.Request) {
	p, err := safePath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, p)
}

// ── 全文搜索（文件名 + 文本内容） ──

var textExts = map[string]bool{
	"md": true, "markdown": true, "txt": true, "go": true, "py": true, "js": true, "ts": true,
	"jsx": true, "tsx": true, "vue": true, "json": true, "yml": true, "yaml": true, "toml": true,
	"sh": true, "bash": true, "html": true, "css": true, "rs": true, "c": true, "h": true,
	"cpp": true, "hpp": true, "java": true, "xml": true, "ini": true, "conf": true, "cfg": true,
	"log": true, "sql": true, "lua": true, "rb": true,
}

var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "__pycache__": true,
	"target": true, "dist": true, "build": true, ".git": true, ".cache": true,
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	scope := strings.Trim(r.URL.Query().Get("path"), "/")
	all := showHidden || r.URL.Query().Get("all") == "1"
	results := []SearchResult{}
	if q == "" {
		writeJSON(w, results)
		return
	}
	base, err := safePath(scope)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	const maxResults = 200
	visited := 0
	filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || len(results) >= maxResults {
			return fs.SkipAll
		}
		visited++
		if visited > 200000 {
			return fs.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if p != base && (skipDirs[name] || (!all && strings.HasPrefix(name, "."))) {
				return filepath.SkipDir
			}
			return nil
		}
		if !all && strings.HasPrefix(name, ".") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		// 文件名匹配
		if strings.Contains(strings.ToLower(name), q) {
			results = append(results, SearchResult{Path: rel, Name: true})
		}
		// 内容匹配（仅文本类型、≤2MB、每文件最多3条）
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if !textExts[ext] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 2<<20 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		matches := 0
		for i, ln := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(ln), q) {
				t := strings.TrimSpace(ln)
				if len(t) > 160 {
					t = t[:160] + "…"
				}
				results = append(results, SearchResult{Path: rel, Line: i + 1, Text: t})
				matches++
				if matches >= 3 || len(results) >= maxResults {
					break
				}
			}
		}
		return nil
	})
	writeJSON(w, results)
}
