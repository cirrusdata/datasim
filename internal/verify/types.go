package verify

import "time"

// EntryType identifies the filesystem entry type used during verification.
type EntryType string

const (
	// EntryTypeFile represents a regular file.
	EntryTypeFile EntryType = "file"
	// EntryTypeDirectory represents a directory.
	EntryTypeDirectory EntryType = "directory"
	// EntryTypeSymlink represents a symbolic link.
	EntryTypeSymlink EntryType = "symlink"
	// EntryTypeOther represents any unsupported special filesystem entry.
	EntryTypeOther EntryType = "other"
)

// DifferenceKind identifies one class of verification mismatch.
type DifferenceKind string

const (
	// DifferenceMissingFromDestination reports that the source entry is absent from the destination tree.
	DifferenceMissingFromDestination DifferenceKind = "missing_from_destination"
	// DifferenceExtraInDestination reports that the destination contains an unexpected extra entry.
	DifferenceExtraInDestination DifferenceKind = "extra_in_destination"
	// DifferenceTypeMismatch reports that source and destination entry types differ.
	DifferenceTypeMismatch DifferenceKind = "entry_type_mismatch"
	// DifferenceUnsupportedType reports that the verifier encountered an unsupported special entry type.
	DifferenceUnsupportedType DifferenceKind = "unsupported_entry_type"
	// DifferenceSizeMismatch reports that regular file sizes differ.
	DifferenceSizeMismatch DifferenceKind = "size_mismatch"
	// DifferenceContentMismatch reports that regular file content hashes differ.
	DifferenceContentMismatch DifferenceKind = "content_mismatch"
	// DifferenceModeMismatch reports that normalized permissions differ.
	DifferenceModeMismatch DifferenceKind = "mode_mismatch"
	// DifferenceModifiedTimeMismatch reports that normalized modification times differ.
	DifferenceModifiedTimeMismatch DifferenceKind = "modified_time_mismatch"
)

// DirOptions describes one directory verification request.
type DirOptions struct {
	SourceRoot      string
	DestinationRoot string
	Metadata        bool
	Excludes        []string
	Workers         int
	Progress        ProgressFunc
}

// Progress describes verifier progress for long-running comparisons.
type Progress struct {
	Phase          string
	CurrentPath    string
	CurrentAction  string
	CompletedItems int
	TotalItems     int
	CompletedBytes int64
	TotalBytes     int64
}

// ProgressFunc receives verification progress updates.
type ProgressFunc func(Progress)

// Result summarizes one verification run.
type Result struct {
	SourceRoot              string       `json:"source_root"`
	DestinationRoot         string       `json:"destination_root"`
	Metadata                bool         `json:"metadata"`
	Excludes                []string     `json:"excludes,omitempty"`
	Matched                 bool         `json:"matched"`
	TotalSourceEntries      int          `json:"total_source_entries"`
	TotalDestinationEntries int          `json:"total_destination_entries"`
	MatchedDirectories      int          `json:"matched_directories"`
	MatchedFiles            int          `json:"matched_files"`
	ComparedFiles           int          `json:"compared_files"`
	ComparedBytes           int64        `json:"compared_bytes"`
	Differences             []Difference `json:"differences,omitempty"`
}

// Difference describes one verification mismatch.
type Difference struct {
	Path        string         `json:"path"`
	Kind        DifferenceKind `json:"kind"`
	Message     string         `json:"message"`
	Source      *Entry         `json:"source,omitempty"`
	Destination *Entry         `json:"destination,omitempty"`
}

// Entry describes one normalized filesystem entry in the verification result.
type Entry struct {
	Path     string    `json:"path"`
	Type     EntryType `json:"type"`
	Size     int64     `json:"size,omitempty"`
	HashMD5  string    `json:"checksum_md5,omitempty"`
	Metadata Metadata  `json:"metadata,omitempty"`
}

// Metadata stores normalized entry metadata used during verification.
type Metadata struct {
	Mode       string            `json:"mode,omitempty"`
	ModifiedAt time.Time         `json:"modified_at,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}
