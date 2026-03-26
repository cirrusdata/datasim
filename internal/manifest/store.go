package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Store struct {
	fileName string
}

// NewStore constructs a manifest store for the given metadata filename.
func NewStore(fileName string) *Store {
	return &Store{fileName: fileName}
}

// Path returns the manifest path for a dataset root.
func (s *Store) Path(root string) string {
	return filepath.Join(root, s.fileName)
}

// Load reads a manifest from a dataset root.
func (s *Store) Load(root string) (*Manifest, error) {
	data, err := os.ReadFile(s.Path(root))
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// Save writes a manifest atomically to a dataset root.
func (s *Store) Save(root string, manifest *Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := s.Path(root)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

// Delete removes the manifest file for a dataset root.
func (s *Store) Delete(root string) error {
	if err := os.Remove(s.Path(root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
