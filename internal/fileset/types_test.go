package fileset

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/cirrusdata/datasim/internal/manifest"
)

// TestPlanInitIsDeterministic verifies repeated planning with the same seed produces the same file inventory.
func TestPlanInitIsDeterministic(t *testing.T) {
	t.Parallel()

	profile, err := NewCatalog().Get("corporate")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	planA, err := profile.PlanInit(context.Background(), InitRequest{
		Root:        t.TempDir(),
		TargetBytes: 8 * 1024 * 1024,
		Seed:        42,
		Strategy:    StrategyBalanced,
	})
	if err != nil {
		t.Fatalf("PlanInit returned error: %v", err)
	}

	planB, err := profile.PlanInit(context.Background(), InitRequest{
		Root:        t.TempDir(),
		TargetBytes: 8 * 1024 * 1024,
		Seed:        42,
		Strategy:    StrategyBalanced,
	})
	if err != nil {
		t.Fatalf("second PlanInit returned error: %v", err)
	}

	if len(planA.Files) < 2 {
		t.Fatalf("expected at least two planned files, got %d", len(planA.Files))
	}
	if len(planA.Files) != len(planB.Files) {
		t.Fatalf("expected matching file counts, got %d and %d", len(planA.Files), len(planB.Files))
	}

	unique := make(map[time.Time]struct{}, len(planA.Files))
	planningClock := deterministicPlanningClock(42, currentGeneratorVersion)
	lowerBound := planningClock.Add(-historicalModifiedAtWindow)
	for index := range planA.Files {
		fileA := planA.Files[index]
		fileB := planB.Files[index]
		if !reflect.DeepEqual(fileA, fileB) {
			t.Fatalf("expected matching file spec at index %d, got %+v and %+v", index, fileA, fileB)
		}
		if fileA.ModifiedAt.Before(lowerBound) {
			t.Fatalf("expected %s mtime to stay within deterministic history window, got %s", fileA.RelativePath, fileA.ModifiedAt)
		}
		if fileA.ModifiedAt.After(planningClock) {
			t.Fatalf("expected %s mtime to be at or before deterministic planning time, got %s", fileA.RelativePath, fileA.ModifiedAt)
		}
		unique[fileA.ModifiedAt] = struct{}{}
	}

	if len(unique) < 2 {
		t.Fatalf("expected varied mtimes across generated files, got %d unique values", len(unique))
	}
}

// TestPlanRotateIsDeterministic verifies repeated rotations with the same seed produce the same mutation plan.
func TestPlanRotateIsDeterministic(t *testing.T) {
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

	planA, err := profile.PlanRotate(context.Background(), RotateRequest{
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
	planB, err := profile.PlanRotate(context.Background(), RotateRequest{
		Manifest:  doc,
		CreatePct: 0,
		DeletePct: 0,
		ModifyPct: 100,
		Seed:      77,
		Strategy:  StrategyBalanced,
	})
	if err != nil {
		t.Fatalf("second PlanRotate returned error: %v", err)
	}

	if len(planA.Mutations) != len(doc.Files) {
		t.Fatalf("expected %d mutations, got %d", len(doc.Files), len(planA.Mutations))
	}
	if len(planA.Mutations) != len(planB.Mutations) {
		t.Fatalf("expected matching mutation counts, got %d and %d", len(planA.Mutations), len(planB.Mutations))
	}

	originals := make(map[string]time.Time, len(doc.Files))
	for _, file := range doc.Files {
		originals[file.Path] = file.ModifiedAt
	}

	rotationClock := deterministicRotationClock(doc, 77)
	for index := range planA.Mutations {
		mutationA := planA.Mutations[index]
		mutationB := planB.Mutations[index]
		if mutationA != mutationB {
			t.Fatalf("expected matching mutation at index %d, got %+v and %+v", index, mutationA, mutationB)
		}

		previous := originals[mutationA.RelativePath]
		if !mutationA.ModifiedAt.After(previous) {
			t.Fatalf("expected %s mtime to advance beyond %s, got %s", mutationA.RelativePath, previous, mutationA.ModifiedAt)
		}
		if mutationA.ModifiedAt.After(rotationClock) {
			t.Fatalf("expected %s mutation mtime to be at or before deterministic rotate time, got %s", mutationA.RelativePath, mutationA.ModifiedAt)
		}
	}
}

// TestDeterministicPlanningClockVerifiesStablePseudoTime verifies the helper returns a stable pseudo-time for a seed.
func TestDeterministicPlanningClockVerifiesStablePseudoTime(t *testing.T) {
	t.Parallel()

	first := deterministicPlanningClock(42, currentGeneratorVersion)
	second := deterministicPlanningClock(42, currentGeneratorVersion)
	third := deterministicPlanningClock(43, currentGeneratorVersion)

	if !first.Equal(second) {
		t.Fatalf("expected identical planning clock values, got %s and %s", first, second)
	}
	if first.Equal(third) {
		t.Fatalf("expected different seeds to produce different planning times, got %s", third)
	}
}

// TestDeterministicRotationClockAdvancesPastLatestMtime verifies rotation time always advances beyond the manifest inventory.
func TestDeterministicRotationClockAdvancesPastLatestMtime(t *testing.T) {
	t.Parallel()

	base := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	doc := &manifest.Manifest{
		History: []manifest.RotationHistory{{}},
		Files: []manifest.FileRecord{
			{Path: "a.txt", ModifiedAt: base},
			{Path: "b.txt", ModifiedAt: base.Add(5 * time.Hour)},
		},
	}

	rotationClock := deterministicRotationClock(doc, 100)
	if !rotationClock.After(base.Add(5 * time.Hour)) {
		t.Fatalf("expected rotation clock to advance beyond latest file mtime, got %s", rotationClock)
	}
}
