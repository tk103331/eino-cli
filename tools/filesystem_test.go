package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
	"gopkg.in/yaml.v3"
)

// newFSTestTool builds a filesystem tool rooted at root. extra lines are
// additional YAML keys under `config:`.
func newFSTestTool(t *testing.T, root string, extra ...string) tool.InvokableTool {
	t.Helper()

	var b strings.Builder
	b.WriteString("type: filesystem\nconfig:\n  root: " + strconv.Quote(root) + "\n")
	for _, line := range extra {
		b.WriteString(line + "\n")
	}

	var cfg config.Tool
	if err := yaml.Unmarshal([]byte(b.String()), &cfg); err != nil {
		t.Fatalf("cannot parse test config: %v", err)
	}
	tl, err := NewFileSystemTool("filesystem", cfg)
	if err != nil {
		t.Fatalf("NewFileSystemTool: %v", err)
	}
	return tl
}

func runFS(t *testing.T, tl tool.InvokableTool, args string) string {
	t.Helper()
	out, err := tl.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected tool error: %v", err)
	}
	return out
}

func TestFileSystemWriteReadAppend(t *testing.T) {
	root := t.TempDir()
	tl := newFSTestTool(t, root)

	if out := runFS(t, tl, `{"operation":"write","path":"hello.txt","content":"hello\nworld"}`); !strings.Contains(out, "wrote 11 bytes") {
		t.Fatalf("write result = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"hello.txt"}`); out != "hello\nworld" {
		t.Fatalf("read result = %q", out)
	}

	if out := runFS(t, tl, `{"operation":"append","path":"hello.txt","content":"\n!"}`); !strings.Contains(out, "appended 2 bytes") {
		t.Fatalf("append result = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"hello.txt"}`); out != "hello\nworld\n!" {
		t.Fatalf("read after append = %q", out)
	}

	if out := runFS(t, tl, `{"operation":"read","path":"hello.txt","offset":2}`); out != "world\n!" {
		t.Fatalf("read with offset = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"hello.txt","offset":2,"limit":1}`); out != "world\n… [1 more lines]" {
		t.Fatalf("read with limit = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"hello.txt","offset":9}`); !strings.Contains(out, "past the end") {
		t.Fatalf("read past end = %q", out)
	}

	// Writes create parent directories on demand.
	if out := runFS(t, tl, `{"operation":"write","path":"a/b/c.txt","content":"x"}`); !strings.Contains(out, "a/b/c.txt") {
		t.Fatalf("nested write = %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(root, "a", "b", "c.txt")); err != nil || string(data) != "x" {
		t.Fatalf("nested file = %q, %v", data, err)
	}

	if out := runFS(t, tl, `{"operation":"write","path":"empty.txt"}`); !strings.Contains(out, "wrote 0 bytes") {
		t.Fatalf("empty write = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"empty.txt"}`); out != "(empty file)" {
		t.Fatalf("empty read = %q", out)
	}
}

func TestFileSystemListStatMkdirMoveDelete(t *testing.T) {
	root := t.TempDir()
	tl := newFSTestTool(t, root)

	runFS(t, tl, `{"operation":"write","path":"src/main.go","content":"12345"}`)
	if out := runFS(t, tl, `{"operation":"mkdir","path":"src/deep"}`); !strings.Contains(out, "created directory src/deep") {
		t.Fatalf("mkdir = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"mkdir","path":"src/deep"}`); !strings.Contains(out, "already exists") {
		t.Fatalf("second mkdir = %q", out)
	}

	out := runFS(t, tl, `{"operation":"list","path":"."}`)
	if !strings.Contains(out, "src/") {
		t.Fatalf("list = %q", out)
	}
	out = runFS(t, tl, `{"operation":"list","path":".","recursive":true}`)
	for _, want := range []string{"src/main.go (5 B)", "src/deep/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("recursive list missing %q:\n%s", want, out)
		}
	}
	if out := runFS(t, tl, `{"operation":"list","path":"src/main.go"}`); !strings.Contains(out, "not a directory") {
		t.Fatalf("list of file = %q", out)
	}

	out = runFS(t, tl, `{"operation":"stat","path":"src/main.go"}`)
	for _, want := range []string{"type: file", "size: 5 bytes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stat missing %q:\n%s", want, out)
		}
	}
	if out := runFS(t, tl, `{"operation":"stat","path":"src"}`); !strings.Contains(out, "type: directory") {
		t.Fatalf("stat dir = %q", out)
	}

	if out := runFS(t, tl, `{"operation":"move","path":"src/main.go","to":"src/main.txt"}`); !strings.Contains(out, "moved src/main.go -> src/main.txt") {
		t.Fatalf("move = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"move","path":"src/main.txt","to":"src/deep"} `); !strings.Contains(out, "already exists") {
		t.Fatalf("move onto existing = %q", out)
	}

	if out := runFS(t, tl, `{"operation":"delete","path":"src"}`); !strings.Contains(out, "not empty") {
		t.Fatalf("delete non-empty dir = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"delete","path":"src","recursive":true}`); out != "deleted src" {
		t.Fatalf("recursive delete = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "src")); !os.IsNotExist(err) {
		t.Fatalf("src should be gone, stat err = %v", err)
	}
	if out := runFS(t, tl, `{"operation":"delete","path":"."}`); !strings.Contains(out, "refusing") {
		t.Fatalf("delete root = %q", out)
	}
}

func TestFileSystemRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	tl := newFSTestTool(t, root)

	if out := runFS(t, tl, fmt.Sprintf(`{"operation":"read","path":%q}`, outsideFile)); !strings.Contains(out, "outside the filesystem root") {
		t.Fatalf("absolute escape = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"../escape.txt"}`); !strings.Contains(out, "outside the filesystem root") {
		t.Fatalf("relative escape = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"~/secret.txt"}`); !strings.Contains(out, "not allowed") {
		t.Fatalf("home escape = %q", out)
	}

	// A symlink inside the root that points outside must not be followed.
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"link/secret.txt"}`); !strings.Contains(out, "resolves outside") {
		t.Fatalf("symlink escape = %q", out)
	}

	// Writing through a dangling symlink would reach an unchecked target.
	dangling := filepath.Join(root, "dangling.txt")
	if err := os.Symlink(filepath.Join(outside, "new.txt"), dangling); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if out := runFS(t, tl, `{"operation":"write","path":"dangling.txt","content":"x"}`); !strings.Contains(out, "broken symlink") {
		t.Fatalf("dangling symlink write = %q", out)
	}

	// Symlinks that stay inside the root keep working.
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(root, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"alias.txt"}`); out != "ok" {
		t.Fatalf("inside symlink = %q", out)
	}
}

func TestFileSystemBinaryAndTruncation(t *testing.T) {
	root := t.TempDir()
	tl := newFSTestTool(t, root, "  max_bytes: 10")

	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte("ab\x00cd"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"bin.dat"}`); !strings.Contains(out, "binary file") {
		t.Fatalf("binary read = %q", out)
	}

	runFS(t, tl, `{"operation":"write","path":"long.txt","content":"0123456789ABCDEF"}`)
	out := runFS(t, tl, `{"operation":"read","path":"long.txt"}`)
	if !strings.Contains(out, "… [truncated at 10 bytes]") || !strings.HasPrefix(out, "0123456789") {
		t.Fatalf("truncated read = %q", out)
	}
}

func TestFileSystemGrep(t *testing.T) {
	root := t.TempDir()
	tl := newFSTestTool(t, root)

	runFS(t, tl, `{"operation":"write","path":"a.go","content":"package main\nfunc main() {}"}`)
	runFS(t, tl, `{"operation":"write","path":"b.txt","content":"func not main"}`)
	runFS(t, tl, `{"operation":"write","path":"sub/c.go","content":"// func x"}`)
	runFS(t, tl, `{"operation":"write","path":"vendor/d.go","content":"func vendored() {}"}`)

	out := runFS(t, tl, `{"operation":"grep","path":".","pattern":"func"}`)
	for _, want := range []string{"a.go:2: func main() {}", "b.txt:1: func not main", "sub/c.go:1: // func x"} {
		if !strings.Contains(out, want) {
			t.Fatalf("grep missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "vendor") {
		t.Fatalf("grep should skip vendor directories:\n%s", out)
	}

	out = runFS(t, tl, `{"operation":"grep","path":".","pattern":"func","glob":"*.txt"}`)
	if !strings.Contains(out, "b.txt:1:") || strings.Contains(out, "a.go") {
		t.Fatalf("grep with glob = %q", out)
	}

	if out := runFS(t, tl, `{"operation":"grep","path":".","pattern":"nothing-here"}`); !strings.Contains(out, "no matches") {
		t.Fatalf("grep no matches = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"grep","path":".","pattern":"("}`); !strings.Contains(out, "invalid regular expression") {
		t.Fatalf("grep invalid regex = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"grep","path":"a.go","pattern":"main"}`); !strings.Contains(out, "a.go:2:") {
		t.Fatalf("grep single file = %q", out)
	}
}

func TestFileSystemArgumentErrors(t *testing.T) {
	tl := newFSTestTool(t, t.TempDir())

	if out := runFS(t, tl, `{"path":"x"}`); !strings.Contains(out, "operation is required") {
		t.Fatalf("missing operation = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"chmod","path":"x"}`); !strings.Contains(out, "unsupported operation") {
		t.Fatalf("unknown operation = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"missing.txt"}`); !strings.Contains(out, "no such file or directory") {
		t.Fatalf("missing file = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"read","path":"."}`); !strings.Contains(out, "is a directory") {
		t.Fatalf("read directory = %q", out)
	}
	if out := runFS(t, tl, `{"operation":"grep","path":"."}`); !strings.Contains(out, "pattern is required") {
		t.Fatalf("grep without pattern = %q", out)
	}

	// Malformed JSON is reported as a result, not as a tool error.
	out, err := tl.InvokableRun(context.Background(), `{`)
	if err != nil {
		t.Fatalf("malformed arguments should not fail the turn: %v", err)
	}
	if !strings.Contains(out, "invalid arguments") {
		t.Fatalf("malformed arguments = %q", out)
	}
}

func TestFileSystemFactoryRegistration(t *testing.T) {
	tl, err := CreateTool("fs", config.Tool{Type: "filesystem"})
	if err != nil {
		t.Fatalf("filesystem type not registered: %v", err)
	}
	info, err := tl.Info(context.Background())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Name != "fs" {
		t.Fatalf("tool name = %q", info.Name)
	}
}
