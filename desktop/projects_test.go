package main

import (
	"errors"
	"go/parser"
	"go/token"
	"path"
	"strings"
	"testing"
)

type projectTestFS struct {
	dirs  map[string]bool
	files map[string]string
	fail  bool
}

func (f *projectTestFS) RealPath(p string) (string, error) {
	if p == "." {
		return "/home/dev", nil
	}
	return p, nil
}
func (f *projectTestFS) Mkdir(p string) error {
	if f.dirs[p] {
		return errors.New("exists")
	}
	f.dirs[p] = true
	return nil
}
func (f *projectTestFS) WriteNewFile(p, content string) error {
	if f.fail {
		return errors.New("disk full")
	}
	if _, ok := f.files[p]; ok {
		return errors.New("exists")
	}
	f.files[p] = content
	return nil
}
func TestProjectCreationTemplatesAndNoOverwrite(t *testing.T) {
	for _, template := range []string{"empty", "go", "node", "python"} {
		fs := &projectTestFS{dirs: map[string]bool{}, files: map[string]string{}}
		result, err := createProject(fs, ".", "my-app", template)
		if err != nil || result.Path != "/home/dev/my-app" {
			t.Fatalf("%s: %+v %v", template, result, err)
		}
		for name := range fs.files {
			if path.Dir(name) != result.Path {
				t.Fatal("file escaped project", name)
			}
		}
		if _, err = createProject(fs, ".", "my-app", template); err == nil {
			t.Fatal("overwrote existing project")
		}
		if template == "go" {
			if _, err = parser.ParseFile(token.NewFileSet(), "main.go", fs.files[result.Path+"/main.go"], parser.AllErrors); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestProjectRejectsTraversalAndKeepsPartialOutput(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../test", "a/b", `a\b`, "a;rm", "a\n", strings.Repeat("a", 65)} {
		if _, err := projectFiles(name, "go"); err == nil {
			t.Fatalf("accepted invalid name %q", name)
		}
	}
	if _, err := projectFiles("app", "unknown"); err == nil {
		t.Fatal("accepted unknown template")
	}
	fs := &projectTestFS{dirs: map[string]bool{}, files: map[string]string{}, fail: true}
	result, err := createProject(fs, "/srv", "app", "go")
	if err == nil || !strings.Contains(err.Error(), "/srv/app") || !fs.dirs[result.Path] {
		t.Fatalf("partial failure not reported/preserved: %+v %v", result, err)
	}
}
