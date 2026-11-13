package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/tools/approval"
)

// File-system operations exposed by the filesystem tool.
const (
	fsOpRead   = "read"
	fsOpWrite  = "write"
	fsOpAppend = "append"
	fsOpList   = "list"
	fsOpStat   = "stat"
	fsOpMkdir  = "mkdir"
	fsOpDelete = "delete"
	fsOpMove   = "move"
	fsOpGrep   = "grep"
)

// Limits used by the filesystem tool.
const (
	fsDefaultMaxBytes   = 200 * 1024
	fsDefaultMaxEntries = 500
	fsDefaultTimeout    = 30 * time.Second
	fsGrepMaxFileSize   = 2 * 1024 * 1024
	fsGrepMaxLineRunes  = 300
)

// fsTool reads and edits files below a single root directory. Every path given
// by the model is resolved against the root and rejected when it escapes it,
// including through symlinks.
//
// Operational failures are returned as the tool result ("error: ...") rather
// than as a Go error: the ReAct loop aborts the whole turn on a tool error,
// while a result string lets the model see the problem and retry with a fixed
// path. Only cancellation is reported as an error.
type fsTool struct {
	info       *schema.ToolInfo
	root       string
	rootReal   string
	maxBytes   int64
	maxEntries int
	timeout    time.Duration
}

// NewFileSystemTool creates the "filesystem" tool.
//
// Config options:
//   - root: directory the tool may access (default: current working directory)
//   - max_bytes: read budget per call in bytes (default 200 KiB)
//   - max_entries: list/grep result cap (default 500 for list, 200 for grep)
//   - timeout: per-call timeout in seconds (default 30)
func NewFileSystemTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	t := &fsTool{
		maxBytes:   fsDefaultMaxBytes,
		maxEntries: fsDefaultMaxEntries,
		timeout:    fsDefaultTimeout,
	}

	if cfg.Config != nil {
		if v, ok := cfg.Config["root"]; ok {
			if root := expandHome(strings.TrimSpace(v.String())); root != "" {
				abs, err := filepath.Abs(root)
				if err != nil {
					return nil, fmt.Errorf("filesystem: invalid root %q: %w", root, err)
				}
				t.root = abs
			}
		}
		if v, ok := cfg.Config["max_bytes"]; ok {
			if n := v.Int(); n > 0 {
				t.maxBytes = int64(n)
			}
		}
		if v, ok := cfg.Config["max_entries"]; ok {
			if n := v.Int(); n > 0 {
				t.maxEntries = n
			}
		}
		if v, ok := cfg.Config["timeout"]; ok {
			if secs := v.Int(); secs > 0 {
				t.timeout = time.Duration(secs) * time.Second
			}
		}
	}

	if t.root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("filesystem: cannot determine working directory: %w", err)
		}
		t.root = wd
	}
	t.root = filepath.Clean(t.root)
	info, err := os.Stat(t.root)
	if err != nil {
		return nil, fmt.Errorf("filesystem: root %q is not accessible: %w", t.root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("filesystem: root %q is not a directory", t.root)
	}
	t.rootReal = t.root
	if real, err := filepath.EvalSymlinks(t.root); err == nil {
		t.rootReal = real
	}

	desc := cfg.Description
	if desc == "" {
		desc = "Read and edit files inside the filesystem root. Supported operations: " +
			"read (text file, optional offset/limit lines), write (create or overwrite), append, " +
			"list (recursive optional), stat, mkdir, delete (non-empty directories need recursive=true), " +
			"move (to=destination) and grep (regular expression, optional glob filter). " +
			"Paths are relative to the root directory; anything outside it is rejected."
	}

	t.info = &schema.ToolInfo{
		Name: name,
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operation": {
				Type:     schema.String,
				Desc:     "Operation to perform.",
				Required: true,
				Enum: []string{
					fsOpRead, fsOpWrite, fsOpAppend, fsOpList,
					fsOpStat, fsOpMkdir, fsOpDelete, fsOpMove, fsOpGrep,
				},
			},
			"path": {
				Type:     schema.String,
				Desc:     "File or directory path, relative to the root. Absolute paths must stay inside the root.",
				Required: true,
			},
			"content": {
				Type: schema.String,
				Desc: "Text to write or append (operation=write or append).",
			},
			"to": {
				Type: schema.String,
				Desc: "Destination path (operation=move).",
			},
			"offset": {
				Type: schema.Integer,
				Desc: "1-based first line to return (operation=read).",
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum lines (read), entries (list) or matches (grep).",
			},
			"recursive": {
				Type: schema.Boolean,
				Desc: "Recurse into subdirectories (operation=list), or allow deleting a non-empty directory (operation=delete).",
			},
			"pattern": {
				Type: schema.String,
				Desc: "Regular expression to search for (operation=grep).",
			},
			"glob": {
				Type: schema.String,
				Desc: "Optional file-name filter for grep, for example *.go.",
			},
		}),
	}
	return t, nil
}

func (t *fsTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// fsInput is the argument set shared by all operations.
type fsInput struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	To        string `json:"to"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Recursive bool   `json:"recursive"`
	Pattern   string `json:"pattern"`
	Glob      string `json:"glob"`
}

func (t *fsTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var in fsInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &in); err != nil {
		return t.fail("invalid arguments: %v", err)
	}

	op := strings.ToLower(strings.TrimSpace(in.Operation))
	if op == "" {
		return t.fail("operation is required (read, write, append, list, stat, mkdir, delete, move, grep)")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	switch op {
	case fsOpRead:
		return t.read(ctx, in)
	case fsOpWrite:
		return t.write(in, false)
	case fsOpAppend:
		return t.write(in, true)
	case fsOpList:
		return t.list(ctx, in)
	case fsOpStat:
		return t.stat(in)
	case fsOpMkdir:
		return t.mkdir(in)
	case fsOpDelete:
		return t.delete(in)
	case fsOpMove:
		return t.move(in)
	case fsOpGrep:
		return t.grep(ctx, in)
	default:
		return t.fail("unsupported operation %q", in.Operation)
	}
}

// ---- helpers ----

// fail reports an operational failure as a tool result so the model can react.
func (t *fsTool) fail(format string, args ...any) (string, error) {
	return "error: " + fmt.Sprintf(format, args...), nil
}

// checkContext reports a deadline as a result (the model can retry) and
// cancellation as an error (the turn really should stop).
func (t *fsTool) checkContext(ctx context.Context) (string, error) {
	if ctx.Err() == nil {
		return "", nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return t.fail("operation timed out after %s", t.timeout)
	}
	return "", ctx.Err()
}

// resolve turns a model-supplied path into an absolute path inside the root.
func (t *fsTool) resolve(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "."
	}
	if strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("path %q is not allowed; use a path relative to the root directory", p)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(t.root, p)
	}
	p = filepath.Clean(p)

	if !withinRoot(t.root, p) {
		return "", fmt.Errorf("path %q is outside the filesystem root %q", p, t.root)
	}

	// Reject symlinks that point outside the root: resolve the target itself,
	// or its nearest existing ancestor when it does not exist yet.
	for check := p; ; {
		real, err := filepath.EvalSymlinks(check)
		if err == nil {
			if !withinRoot(t.rootReal, real) {
				return "", fmt.Errorf("path %q resolves outside the filesystem root %q", p, t.root)
			}
			break
		}
		parent := filepath.Dir(check)
		if parent == check {
			break
		}
		check = parent
	}

	// A dangling symlink would be followed by open/create to a target that was
	// never checked, so refuse it. Existing links were validated above.
	if info, err := os.Lstat(p); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if _, err := filepath.EvalSymlinks(p); err != nil {
			return "", fmt.Errorf("path %q is a broken symlink", p)
		}
	}
	return p, nil
}

// withinRoot reports whether p is root itself or lies below it.
func withinRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// display renders a path relative to the root for tool results.
func (t *fsTool) display(p string) string {
	rel, err := filepath.Rel(t.root, p)
	if err != nil || rel == "" {
		return p
	}
	return filepath.ToSlash(rel)
}

// osError turns a file-system error into a short, model-friendly message.
func (t *fsTool) osError(p string, err error) string {
	name := t.display(p)
	switch {
	case os.IsNotExist(err):
		return name + ": no such file or directory"
	case os.IsPermission(err):
		return name + ": permission denied"
	default:
		return fmt.Sprintf("%s: %v", name, err)
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// ApprovalRequest flags only mutating operations: reading and searching are
// harmless, while write/append/mkdir/delete/move change the disk.
func (t *fsTool) ApprovalRequest(arguments string) (approval.Request, bool) {
	var in fsInput
	if err := json.Unmarshal([]byte(arguments), &in); err != nil {
		return approval.Request{}, false
	}
	op := strings.ToLower(strings.TrimSpace(in.Operation))
	path := strings.TrimSpace(in.Path)
	if path == "" {
		path = "."
	}

	var summary string
	switch op {
	case fsOpWrite:
		summary = fmt.Sprintf("写入文件 %s（%d 字节）", path, len(in.Content))
	case fsOpAppend:
		summary = fmt.Sprintf("追加内容到 %s（%d 字节）", path, len(in.Content))
	case fsOpMkdir:
		summary = "创建目录 " + path
	case fsOpDelete:
		summary = "删除 " + path
		if in.Recursive {
			summary += "（递归）"
		}
	case fsOpMove:
		summary = fmt.Sprintf("移动 %s -> %s", path, strings.TrimSpace(in.To))
	default:
		return approval.Request{}, false
	}
	return approval.Request{Tool: t.info.Name, Summary: summary, Detail: arguments}, true
}

// ---- operations ----

func (t *fsTool) read(ctx context.Context, in fsInput) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	if res, err := t.checkContext(ctx); err != nil || res != "" {
		return res, err
	}

	f, err := os.Open(p)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	if info.IsDir() {
		return t.fail("%s is a directory (use operation=list)", t.display(p))
	}

	data, err := io.ReadAll(io.LimitReader(f, t.maxBytes+1))
	if err != nil {
		return t.fail("cannot read %s: %v", t.display(p), err)
	}
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return t.fail("%s looks like a binary file (%d bytes); this tool only handles text", t.display(p), info.Size())
	}

	truncated := int64(len(data)) > t.maxBytes
	if truncated {
		data = data[:t.maxBytes]
	}
	text := strings.TrimSuffix(strings.ToValidUTF8(string(data), ""), "\n")
	if text == "" {
		return "(empty file)", nil
	}

	lines := strings.Split(text, "\n")
	offset := in.Offset
	if offset < 1 {
		offset = 1
	}
	if offset > len(lines) {
		return t.fail("offset %d is past the end of %s (%d lines)", offset, t.display(p), len(lines))
	}
	visible := lines[offset-1:]

	limit := in.Limit
	if limit <= 0 {
		limit = 2000
	}
	moreLines := 0
	if limit < len(visible) {
		moreLines = len(visible) - limit
		visible = visible[:limit]
	}

	out := strings.Join(visible, "\n")
	if moreLines > 0 {
		out += fmt.Sprintf("\n… [%d more lines]", moreLines)
	}
	if truncated {
		out += fmt.Sprintf("\n… [truncated at %d bytes]", t.maxBytes)
	}
	return out, nil
}

func (t *fsTool) write(in fsInput, appendOp bool) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	if p == t.root {
		return t.fail("cannot write to the root directory")
	}
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return t.fail("%s is a directory", t.display(p))
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return t.fail("cannot create parent directory for %s: %v", t.display(p), err)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendOp {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(p, flags, 0o644)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	n, writeErr := io.WriteString(f, in.Content)
	closeErr := f.Close()
	if writeErr != nil {
		return t.fail("cannot write %s: %v", t.display(p), writeErr)
	}
	if closeErr != nil {
		return t.fail("cannot write %s: %v", t.display(p), closeErr)
	}

	verb := "wrote"
	if appendOp {
		verb = "appended"
	}
	return fmt.Sprintf("%s %d bytes to %s", verb, n, t.display(p)), nil
}

func (t *fsTool) list(ctx context.Context, in fsInput) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	if !info.IsDir() {
		return t.fail("%s is not a directory (use operation=stat)", t.display(p))
	}

	limit := in.Limit
	if limit <= 0 {
		limit = t.maxEntries
	}

	type entry struct {
		path string
		dir  bool
		size int64
	}
	var entries []entry
	more := false

	add := func(path string, d fs.DirEntry) {
		info, err := d.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		entries = append(entries, entry{path: path, dir: d.IsDir(), size: size})
	}

	if in.Recursive {
		walkErr := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || path == p {
				return nil // skip unreadable entries instead of failing the listing
			}
			if len(entries) >= limit {
				more = true
				return filepath.SkipAll
			}
			add(path, d)
			return nil
		})
		if walkErr != nil && ctx.Err() != nil {
			if res, err := t.checkContext(ctx); err != nil || res != "" {
				return res, err
			}
		}
	} else {
		items, err := os.ReadDir(p)
		if err != nil {
			return t.fail("%s", t.osError(p, err))
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
		for _, item := range items {
			if len(entries) >= limit {
				more = true
				break
			}
			add(filepath.Join(p, item.Name()), item)
		}
	}

	if len(entries) == 0 {
		return "(empty directory)", nil
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.dir {
			lines = append(lines, t.display(e.path)+"/")
		} else {
			lines = append(lines, fmt.Sprintf("%s (%s)", t.display(e.path), humanSize(e.size)))
		}
	}
	out := strings.Join(lines, "\n")
	if more {
		out += fmt.Sprintf("\n… [stopped after %d entries]", len(entries))
	}
	return out, nil
}

func (t *fsTool) stat(in fsInput) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	info, err := os.Lstat(p)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}

	kind := "file"
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		kind = "symlink"
	case info.IsDir():
		kind = "directory"
	}
	lines := []string{
		t.display(p),
		"type: " + kind,
		fmt.Sprintf("size: %d bytes", info.Size()),
		"mode: " + info.Mode().String(),
		"modified: " + info.ModTime().Format(time.RFC3339),
	}
	if kind == "symlink" {
		if target, err := os.Readlink(p); err == nil {
			lines = append(lines, "-> "+target)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (t *fsTool) mkdir(in fsInput) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return "already exists: " + t.display(p), nil
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		return t.fail("cannot create %s: %v", t.display(p), err)
	}
	return "created directory " + t.display(p), nil
}

func (t *fsTool) delete(in fsInput) (string, error) {
	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	if p == t.root {
		return t.fail("refusing to delete the filesystem root")
	}
	info, err := os.Lstat(p)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	if info.IsDir() && !in.Recursive {
		items, err := os.ReadDir(p)
		if err == nil && len(items) > 0 {
			return t.fail("directory %s is not empty; pass recursive=true to delete it", t.display(p))
		}
	}

	if in.Recursive {
		err = os.RemoveAll(p)
	} else {
		err = os.Remove(p)
	}
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}
	return "deleted " + t.display(p), nil
}

func (t *fsTool) move(in fsInput) (string, error) {
	src, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	dst, err := t.resolve(in.To)
	if err != nil {
		return t.fail("%v", err)
	}
	if src == t.root || dst == t.root {
		return t.fail("cannot move the filesystem root")
	}
	if _, err := os.Lstat(dst); err == nil {
		return t.fail("destination %s already exists", t.display(dst))
	}
	if _, err := os.Lstat(src); err != nil {
		return t.fail("%s", t.osError(src, err))
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return t.fail("cannot create destination directory for %s: %v", t.display(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return t.fail("cannot move %s to %s: %v", t.display(src), t.display(dst), err)
	}
	return fmt.Sprintf("moved %s -> %s", t.display(src), t.display(dst)), nil
}

func (t *fsTool) grep(ctx context.Context, in fsInput) (string, error) {
	if strings.TrimSpace(in.Pattern) == "" {
		return t.fail("pattern is required for operation=grep")
	}
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return t.fail("invalid regular expression %q: %v", in.Pattern, err)
	}

	p, err := t.resolve(in.Path)
	if err != nil {
		return t.fail("%v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		return t.fail("%s", t.osError(p, err))
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 200
	}
	var matches []string
	truncated := false

	searchFile := func(path string) error {
		if len(matches) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil || fileInfo.IsDir() || fileInfo.Size() > fsGrepMaxFileSize {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(f, fsGrepMaxFileSize))
		if err != nil {
			return nil
		}
		head := data
		if len(head) > 4096 {
			head = head[:4096]
		}
		if bytes.IndexByte(head, 0) >= 0 {
			return nil // skip binary files
		}

		for i, line := range strings.Split(strings.ToValidUTF8(string(data), ""), "\n") {
			if !re.MatchString(line) {
				continue
			}
			if len(matches) >= limit {
				truncated = true
				return filepath.SkipAll
			}
			matches = append(matches, fmt.Sprintf("%s:%d: %s", t.display(path), i+1, truncateRunes(line, fsGrepMaxLineRunes)))
		}
		return nil
	}

	if info.IsDir() {
		walkErr := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != p && skipGrepDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if in.Glob != "" {
				if ok, _ := filepath.Match(in.Glob, d.Name()); !ok {
					return nil
				}
			}
			return searchFile(path)
		})
		if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
			if res, err := t.checkContext(ctx); err != nil || res != "" {
				return res, err
			}
			return t.fail("cannot search %s: %v", t.display(p), walkErr)
		}
	} else {
		if err := searchFile(p); err != nil {
			if res, err := t.checkContext(ctx); err != nil || res != "" {
				return res, err
			}
			return t.fail("cannot search %s: %v", t.display(p), err)
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("no matches for %q in %s", in.Pattern, t.display(p)), nil
	}
	out := strings.Join(matches, "\n")
	if truncated {
		out += fmt.Sprintf("\n… [stopped after %d matches]", len(matches))
	}
	return out, nil
}

// skipGrepDir lists directories that are never searched by grep: VCS data,
// dependencies and build output would drown real matches.
func skipGrepDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", ".idea", ".next", ".cache", ".gradle", "dist", "build", "target":
		return true
	}
	return false
}

// humanSize formats a byte count for directory listings.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, u := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, u)
		}
	}
	return fmt.Sprintf("%.1f PB", value)
}

// truncateRunes shortens a line for grep output.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
