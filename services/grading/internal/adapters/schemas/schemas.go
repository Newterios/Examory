// Package schemas embeds the event JSON Schemas grading validates at runtime.
// The files are copies of docs/03_architecture/schemas (the Docker build
// context cannot reach docs/); TestCopiesMatchDocs fails if they drift.
// Refresh with `make sync-schemas` from the repository root.
package schemas

import (
	"embed"

	"github.com/Newterios/Examory/services/platform/eventschema"
)

//go:embed *.schema.json
var files embed.FS

// Validator compiles the embedded schemas.
func Validator() (*eventschema.Validator, error) { return eventschema.LoadFS(files) }
