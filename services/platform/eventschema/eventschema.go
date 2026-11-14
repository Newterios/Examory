// Package eventschema validates serialized events against the JSON Schemas
// in docs/03_architecture/schemas (draft 2020-12). Producers use it in
// contract tests (Load/FindDir); consumers that must reject invalid input at
// runtime embed a copy of the schemas and use LoadFS.
package eventschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const baseURL = "https://examory.dev/schemas/"

// Validator holds compiled schemas.
type Validator struct {
	mu       sync.Mutex
	compiler *jsonschema.Compiler
	cache    map[string]*jsonschema.Schema
}

// FindDir walks up from the working directory to the repository's
// docs/03_architecture/schemas directory.
func FindDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "docs", "03_architecture", "schemas")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("eventschema: docs/03_architecture/schemas not found above working directory")
		}
		dir = parent
	}
}

// Load compiles every *.schema.json in dir.
func Load(dir string) (*Validator, error) { return LoadFS(os.DirFS(dir)) }

// LoadFS compiles every top-level *.schema.json of fsys (e.g. an embed.FS).
func LoadFS(fsys fs.FS) (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	files, err := fs.Glob(fsys, "*.schema.json")
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("eventschema: no *.schema.json files")
	}
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("eventschema: %s: %w", f, err)
		}
		if err := c.AddResource(baseURL+path.Base(f), doc); err != nil {
			return nil, err
		}
	}
	return &Validator{compiler: c, cache: map[string]*jsonschema.Schema{}}, nil
}

// Has reports whether a schema exists for the event type and version.
func (v *Validator) Has(eventType string, version int) bool {
	_, err := v.schema(eventType, version)
	return err == nil
}

func (v *Validator) schema(eventType string, version int) (*jsonschema.Schema, error) {
	name := fmt.Sprintf("%s.v%d.schema.json", eventType, version)
	v.mu.Lock()
	defer v.mu.Unlock()
	if sch, ok := v.cache[name]; ok {
		return sch, nil
	}
	sch, err := v.compiler.Compile(baseURL + name)
	if err != nil {
		return nil, fmt.Errorf("eventschema: no schema for %s v%d: %w", eventType, version, err)
	}
	v.cache[name] = sch
	return sch, nil
}

// Validate checks one serialized event against the schema selected by its
// event_type and version ("<event_type>.v<version>.schema.json").
func (v *Validator) Validate(event []byte) error {
	var head struct {
		EventType string `json:"event_type"`
		Version   int    `json:"version"`
	}
	if err := json.Unmarshal(event, &head); err != nil {
		return fmt.Errorf("eventschema: decode: %w", err)
	}
	if head.EventType == "" || strings.ContainsAny(head.EventType, "/\\") {
		return fmt.Errorf("eventschema: missing or invalid event_type")
	}
	sch, err := v.schema(head.EventType, head.Version)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(event))
	if err != nil {
		return err
	}
	return sch.Validate(inst)
}
