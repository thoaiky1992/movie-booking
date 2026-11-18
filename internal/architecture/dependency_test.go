package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCleanArchitectureDependencyDirection(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	rules := []struct {
		layer     string
		forbidden []string
	}{
		{
			layer: "domain",
			forbidden: []string{
				"movie-booking/modules/", "/application", "/infrastructure", "/presentation",
				"github.com/gin-gonic", "github.com/redis", "github.com/hibiken",
				"gorm.io/", "database/sql",
			},
		},
		{
			layer: "application",
			forbidden: []string{
				"/infrastructure", "/presentation", "github.com/gin-gonic",
				"github.com/redis", "github.com/hibiken", "gorm.io/", "database/sql",
			},
		},
		{
			layer:     "infrastructure",
			forbidden: []string{"/presentation"},
		},
		{
			layer:     "presentation",
			forbidden: []string{"/infrastructure"},
		},
	}

	for _, rule := range rules {
		rule := rule
		t.Run(rule.layer, func(t *testing.T) {
			pattern := filepath.Join(root, "modules", "*", rule.layer)
			directories, err := filepath.Glob(pattern)
			if err != nil {
				t.Fatal(err)
			}
			for _, directory := range directories {
				assertImportsDoNotContain(t, directory, rule.forbidden)
			}
		})
	}
}

func assertImportsDoNotContain(t *testing.T, directory string, forbidden []string) {
	t.Helper()
	_ = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("walk %s: %v", path, err)
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Errorf("parse %s: %v", path, parseErr)
			return nil
		}
		for _, imported := range file.Imports {
			name, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Errorf("read import in %s: %v", path, unquoteErr)
				continue
			}
			for _, fragment := range forbidden {
				if strings.Contains(name, fragment) {
					t.Errorf("%s imports forbidden dependency %q", path, name)
				}
			}
		}
		return nil
	})
}
