package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkDirFromArgs(t *testing.T) {
	if got, err := workDirFromArgs(nil, ""); err != nil || got != "" {
		t.Fatalf("no args = %q, %v", got, err)
	}
	if got, err := workDirFromArgs(nil, "/flag"); err != nil || got != "/flag" {
		t.Fatalf("flag only = %q, %v", got, err)
	}
	if got, err := workDirFromArgs([]string{"/pos"}, ""); err != nil || got != "/pos" {
		t.Fatalf("positional = %q, %v", got, err)
	}
	if _, err := workDirFromArgs([]string{"/pos"}, "/flag"); err == nil {
		t.Fatal("positional and --cwd together must fail")
	}
	if _, err := workDirFromArgs([]string{"/a", "/b"}, ""); err == nil {
		t.Fatal("more than one positional directory must fail")
	}
}

func TestEnterWorkDir(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}

	// Empty keeps the current directory.
	if err := enterWorkDir(""); err != nil {
		t.Fatalf("empty work dir: %v", err)
	}
	if wd, _ := os.Getwd(); resolve(wd) != resolve(original) {
		t.Fatalf("empty work dir changed cwd to %s", wd)
	}

	// A target directory is entered.
	target := t.TempDir()
	if err := enterWorkDir(target); err != nil {
		t.Fatalf("enter %s: %v", target, err)
	}
	if wd, _ := os.Getwd(); resolve(wd) != resolve(target) {
		t.Fatalf("cwd = %s, want %s", wd, target)
	}

	// A missing directory is an error, and the cwd is kept.
	if err := enterWorkDir(filepath.Join(target, "missing")); err == nil {
		t.Fatal("missing work dir must fail")
	}
	if wd, _ := os.Getwd(); resolve(wd) != resolve(target) {
		t.Fatalf("failed chdir changed cwd to %s", wd)
	}

	// "~" expands to the home directory.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}
	if err := enterWorkDir("~"); err != nil {
		t.Fatalf("enter ~: %v", err)
	}
	if wd, _ := os.Getwd(); resolve(wd) != resolve(home) {
		t.Fatalf("cwd = %s, want home %s", wd, home)
	}
}
