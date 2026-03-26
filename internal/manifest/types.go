package manifest

import "time"

type Manifest struct {
	Version    int               `json:"version"`
	Workload   string            `json:"workload"`
	Profile    string            `json:"profile,omitempty"`
	Strategy   string            `json:"strategy"`
	Seed       int64             `json:"seed"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Filesystem Filesystem        `json:"filesystem"`
	Generation Generation        `json:"generation"`
	Status     Status            `json:"status"`
	Files      []FileRecord      `json:"files"`
	History    []RotationHistory `json:"history,omitempty"`
}

type Filesystem struct {
	Root string `json:"root"`
}

type Generation struct {
	TargetBytes           int64   `json:"target_bytes"`
	DefaultedFromCapacity bool    `json:"defaulted_from_capacity"`
	CapacityBytes         uint64  `json:"capacity_bytes,omitempty"`
	TargetUtilizationPct  float64 `json:"target_utilization_pct,omitempty"`
}

type Status struct {
	State          string         `json:"state"`
	FileCount      int            `json:"file_count"`
	TotalBytes     int64          `json:"total_bytes"`
	CategoryCounts map[string]int `json:"category_counts,omitempty"`
	RotationCount  int            `json:"rotation_count"`
	LastAction     string         `json:"last_action"`
	LastActionAt   time.Time      `json:"last_action_at"`
	LastCreated    int            `json:"last_created,omitempty"`
	LastDeleted    int            `json:"last_deleted,omitempty"`
	LastModified   int            `json:"last_modified,omitempty"`
}

type FileRecord struct {
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	ChecksumMD5 string            `json:"checksum_md5"`
	Mode        string            `json:"mode"`
	ModifiedAt  time.Time         `json:"modified_at"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type RotationHistory struct {
	At        time.Time `json:"at"`
	Seed      int64     `json:"seed"`
	CreatePct float64   `json:"create_pct"`
	DeletePct float64   `json:"delete_pct"`
	ModifyPct float64   `json:"modify_pct"`
	Created   int       `json:"created"`
	Deleted   int       `json:"deleted"`
	Modified  int       `json:"modified"`
	Strategy  string    `json:"strategy"`
}
