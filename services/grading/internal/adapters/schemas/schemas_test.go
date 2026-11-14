package schemas

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Newterios/Examory/services/platform/eventschema"
)

func TestCopiesMatchDocs(t *testing.T) {
	dir, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	names, _ := fs.Glob(files, "*.schema.json")
	if len(names) == 0 {
		t.Fatal("no embedded schemas")
	}
	for _, n := range names {
		embedded, _ := fs.ReadFile(files, n)
		doc, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil || !bytes.Equal(embedded, doc) {
			t.Errorf("%s differs from docs/03_architecture/schemas (run make sync-schemas)", n)
		}
	}
	if _, err := Validator(); err != nil {
		t.Fatal(err)
	}
}
