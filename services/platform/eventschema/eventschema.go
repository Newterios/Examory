// Package eventschema validates serialized events against the JSON Schemas
// in docs/03_architecture/schemas (draft 2020-12). It is intended for
// contract tests; production binaries should not import it.
package eventschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
func Load(dir string) (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("eventschema: no schemas in %s", dir)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f) // #nosec G304 -- test-only; paths come from a glob of the schema directory
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("eventschema: %s: %w", f, err)
		}
		if err := c.AddResource(baseURL+filepath.Base(f), doc); err != nil {
			return nil, err
		}
	}
	return &Validator{compiler: c, cache: map[string]*jsonschema.Schema{}}, nil
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
	name := fmt.Sprintf("%s.v%d.schema.json", head.EventType, head.Version)
	v.mu.Lock()
	sch, ok := v.cache[name]
	if !ok {
		var err error
		sch, err = v.compiler.Compile(baseURL + name)
		if err != nil {
			v.mu.Unlock()
			return fmt.Errorf("eventschema: no schema for %s v%d: %w", head.EventType, head.Version, err)
		}
		v.cache[name] = sch
	}
	v.mu.Unlock()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(event))
	if err != nil {
		return err
	}
	return sch.Validate(inst)
}
