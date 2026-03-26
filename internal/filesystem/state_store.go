package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/cirrusdata/datasim/internal/config"
)

type StateStore struct {
	path string
}

type stateDocument struct {
	Filesystems []FilesystemRecord `json:"filesystems"`
}

// NewStateStore constructs the filesystem state store.
func NewStateStore(cfg config.Config) (*StateStore, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.StateFile), 0o755); err != nil {
		return nil, err
	}

	return &StateStore{path: cfg.StateFile}, nil
}

// Get looks up a filesystem record by mount point.
func (s *StateStore) Get(mountPoint string) (FilesystemRecord, bool, error) {
	doc, err := s.load()
	if err != nil {
		return FilesystemRecord{}, false, err
	}

	for _, record := range doc.Filesystems {
		if record.MountPoint == mountPoint {
			return record, true, nil
		}
	}

	return FilesystemRecord{}, false, nil
}

// GetByBlockDevice looks up a filesystem record by block-device path.
func (s *StateStore) GetByBlockDevice(blockDevice string) (FilesystemRecord, bool, error) {
	doc, err := s.load()
	if err != nil {
		return FilesystemRecord{}, false, err
	}

	for _, record := range doc.Filesystems {
		if record.BlockDevice == blockDevice {
			return record, true, nil
		}
	}

	return FilesystemRecord{}, false, nil
}

// Put inserts or replaces a filesystem record.
func (s *StateStore) Put(record FilesystemRecord) error {
	doc, err := s.load()
	if err != nil {
		return err
	}

	doc.Filesystems = slices.DeleteFunc(doc.Filesystems, func(existing FilesystemRecord) bool {
		return existing.MountPoint == record.MountPoint
	})
	doc.Filesystems = append(doc.Filesystems, record)

	return s.save(doc)
}

// DeleteByBlockDevice removes filesystem records by block-device path.
func (s *StateStore) DeleteByBlockDevice(blockDevice string) error {
	doc, err := s.load()
	if err != nil {
		return err
	}

	doc.Filesystems = slices.DeleteFunc(doc.Filesystems, func(existing FilesystemRecord) bool {
		return existing.BlockDevice == blockDevice
	})

	return s.save(doc)
}

// Delete removes a filesystem record by mount point.
func (s *StateStore) Delete(mountPoint string) error {
	doc, err := s.load()
	if err != nil {
		return err
	}

	doc.Filesystems = slices.DeleteFunc(doc.Filesystems, func(existing FilesystemRecord) bool {
		return existing.MountPoint == mountPoint
	})

	return s.save(doc)
}

// load reads the current state document from disk.
func (s *StateStore) load() (stateDocument, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return stateDocument{}, nil
		}
		return stateDocument{}, err
	}

	var doc stateDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return stateDocument{}, err
	}

	return doc, nil
}

// save writes the state document atomically.
func (s *StateStore) save(doc stateDocument) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, s.path)
}
