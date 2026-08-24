package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type fileFormat struct {
	Edits []*Edit `json:"edits"`
}

// LoadDocument reads a document from a .bdraw.json file at path.
func LoadDocument(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	d := NewDocument()
	d.Path = path
	d.Edits = f.Edits
	for _, e := range d.Edits {
		if e.ID > d.nextID {
			d.nextID = e.ID
		}
	}
	return d, nil
}

// Save writes the document as JSON to path.
func (d *Document) Save(path string) error {
	data, err := json.MarshalIndent(fileFormat{Edits: d.Edits}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	d.Path = path
	d.Dirty = false
	return nil
}

// IsPNGPath reports whether path names a PNG export target rather than a
// JSON document.
func IsPNGPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".png")
}
