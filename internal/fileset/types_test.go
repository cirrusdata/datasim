package fileset

import (
	"context"
	"testing"
	"time"

	"github.com/cirrusdata/datasim/internal/manifest"
)

// TestPlanInitRandomizesModifiedDates verifies synthetic files receive varied historical mtimes.
func TestPlanInitRandomizesModifiedDates(t *testing.T) {
	t.Parallel()

	profile, err := NewCatalog().Get("corporate")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	before := time.Now().UTC()
	plan, err := profile.PlanInit(context.Background(), InitRequest{
		Root:        t.TempDir(),
		TargetBytes: 8 * 1024 * 1024,
		Seed:        42,
		Strategy:    StrategyBalanced,
	})
	if err != nil {
		t.Fatalf("PlanInit returned error: %v", err)
	}
	after := time.Now().UTC()

	if len(plan.Files) < 2 {
		t.Fatalf("expected at least two planned files, got %d", len(plan.Files))
	}

	unique := make(map[time.Time]struct{}, len(plan.Files))
	for _, file := range plan.Files {
		if file.ModifiedAt.Before(before.Add(-historicalModifiedAtWindow)) {
			t.Fatalf("expected %s mtime to stay within randomized history window, got %s", file.RelativePath, file.ModifiedAt)
		}
		if file.ModifiedAt.After(after) {
			t.Fatalf("expected %s mtime to be at or before planning time, got %s", file.RelativePath, file.ModifiedAt)
		}
		unique[file.ModifiedAt] = struct{}{}
	}

	if len(unique) < 2 {
		t.Fatalf("expected varied mtimes across generated files, got %d unique values", len(unique))
	}
}

// TestPlanRotateRandomizesMutationDates verifies modified files keep advancing to varied mtimes.
func TestPlanRotateRandomizesMutationDates(t *testing.T) {
	t.Parallel()

	profile, err := NewCatalog().Get("corporate")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	base := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	doc := &manifest.Manifest{
		Filesystem: manifest.Filesystem{Root: t.TempDir()},
		Files: []manifest.FileRecord{
			{Path: "finance/report-1.xlsx", Size: 4096, ModifiedAt: base},
			{Path: "engineering/design-2.docx", Size: 8192, ModifiedAt: base.Add(2 * time.Hour)},
			{Path: "shared/ops/log-3.txt", Size: 2048, ModifiedAt: base.Add(4 * time.Hour)},
		},
	}

	plan, err := profile.PlanRotate(context.Background(), RotateRequest{
		Manifest:  doc,
		CreatePct: 0,
		DeletePct: 0,
		ModifyPct: 100,
		Seed:      77,
		Strategy:  StrategyBalanced,
	})
	if err != nil {
		t.Fatalf("PlanRotate returned error: %v", err)
	}
	after := time.Now().UTC()

	if len(plan.Mutations) != len(doc.Files) {
		t.Fatalf("expected %d mutations, got %d", len(doc.Files), len(plan.Mutations))
	}

	originals := make(map[string]time.Time, len(doc.Files))
	for _, file := range doc.Files {
		originals[file.Path] = file.ModifiedAt
	}

	for _, mutation := range plan.Mutations {
		previous := originals[mutation.RelativePath]
		if !mutation.ModifiedAt.After(previous) {
			t.Fatalf("expected %s mtime to advance beyond %s, got %s", mutation.RelativePath, previous, mutation.ModifiedAt)
		}
		if mutation.ModifiedAt.After(after) {
			t.Fatalf("expected %s mutation mtime to be at or before rotate completion, got %s", mutation.RelativePath, mutation.ModifiedAt)
		}
	}
}
