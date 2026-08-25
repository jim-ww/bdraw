package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CurrentFileVersion is written into every saved .bdraw.json file. Bump it
// whenever the file format changes in a way that needs migration logic.
const CurrentFileVersion = 1

type fileFormat struct {
	Version int     `json:"version"`
	Edits   []*Edit `json:"edits"`
}

// LoadDocument reads a document from a .bdraw.json file at path. Files
// saved before versioning existed simply have no "version" field, which
// unmarshals as 0 — that's fine, there's nothing to migrate yet.
func LoadDocument(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Version > CurrentFileVersion {
		return nil, fmt.Errorf("%s was saved by a newer version of bdraw (file version %d, this build supports up to %d)", path, f.Version, CurrentFileVersion)
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

// marshalFileFormat is the JSON encoding shared by Save and autosave.
func marshalFileFormat(d *Document) ([]byte, error) {
	return json.MarshalIndent(fileFormat{Version: CurrentFileVersion, Edits: d.Edits}, "", "  ")
}

// Save writes the document as JSON to path.
func (d *Document) Save(path string) error {
	data, err := marshalFileFormat(d)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	d.Path = path
	d.Dirty = false
	clearAutosave(d)
	return nil
}

// IsPNGPath reports whether path names a PNG export target rather than a
// JSON document.
func IsPNGPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".png")
}

// IsSVGPath reports whether path names an SVG export target.
func IsSVGPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".svg")
}
