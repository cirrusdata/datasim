package fileset

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cirrusdata/datasim/internal/manifest"
)

// TestInitAndRotate verifies fileset initialization and rotation manifest updates.
func TestInitAndRotate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := manifest.NewStore(".cirrusdata-datasim")
	service := NewService(NewCatalog(), store)

	doc, err := service.Init(context.Background(), InitOptions{
		Profile:   "corporate",
		Root:      root,
		TotalSize: "1MiB",
		Seed:      99,
		Strategy:  "balanced",
		Workers:   4,
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if len(doc.Files) == 0 {
		t.Fatal("expected manifest to contain files")
	}

	if doc.Workload != "fileset" {
		t.Fatalf("expected workload to be fileset, got %q", doc.Workload)
	}

	manifestPath := filepath.Join(root, ".cirrusdata-datasim")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest file to exist: %v", err)
	}

	rotated, err := service.Rotate(context.Background(), RotateOptions{
		Root:      root,
		CreatePct: 5,
		DeletePct: 5,
		ModifyPct: 10,
		Seed:      101,
		Strategy:  "balanced",
		Workers:   4,
	})
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}

	if len(rotated.History) != 1 {
		t.Fatalf("expected one rotation history record, got %d", len(rotated.History))
	}

	if rotated.Status.LastAction != "rotate" {
		t.Fatalf("expected last action to be rotate, got %q", rotated.Status.LastAction)
	}

	if err := service.Destroy(DestroyOptions{Root: root}); err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}

	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("expected manifest to be removed, got err=%v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read destroyed root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected destroy to leave root empty, found %d entries", len(entries))
	}
}

// TestProgressCallbacks verifies that long-running fileset operations emit progress.
func TestProgressCallbacks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := manifest.NewStore(".cirrusdata-datasim")
	service := NewService(NewCatalog(), store)

	initCalls := 0
	lastInit := Progress{}
	_, err := service.Init(context.Background(), InitOptions{
		Profile:   "corporate",
		Root:      root,
		TotalSize: "1MiB",
		Seed:      99,
		Strategy:  StrategyBalanced,
		Workers:   4,
		Progress: func(progress Progress) {
			initCalls++
			lastInit = progress
		},
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if initCalls == 0 {
		t.Fatal("expected init progress callbacks")
	}
	if lastInit.Phase != "save" || lastInit.CompletedItems != 1 || lastInit.TotalItems != 1 {
		t.Fatalf("expected init to finish with manifest save progress, got %+v", lastInit)
	}

	rotateCalls := 0
	lastRotate := Progress{}
	_, err = service.Rotate(context.Background(), RotateOptions{
		Root:      root,
		CreatePct: 5,
		DeletePct: 5,
		ModifyPct: 10,
		Seed:      101,
		Strategy:  StrategyBalanced,
		Workers:   4,
		Progress: func(progress Progress) {
			rotateCalls++
			lastRotate = progress
		},
	})
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if rotateCalls == 0 {
		t.Fatal("expected rotate progress callbacks")
	}
	if lastRotate.Phase != "save" || lastRotate.CompletedItems != 1 || lastRotate.TotalItems != 1 {
		t.Fatalf("expected rotate to finish with manifest save progress, got %+v", lastRotate)
	}

	destroyCalls := 0
	lastDestroy := Progress{}
	err = service.Destroy(DestroyOptions{
		Root: root,
		Progress: func(progress Progress) {
			destroyCalls++
			lastDestroy = progress
		},
	})
	if err != nil {
		t.Fatalf("Destroy returned error: %v", err)
	}
	if destroyCalls == 0 {
		t.Fatal("expected destroy progress callbacks")
	}
	if lastDestroy.Phase != "save" || lastDestroy.CompletedItems != 1 || lastDestroy.TotalItems != 1 {
		t.Fatalf("expected destroy to finish with manifest delete progress, got %+v", lastDestroy)
	}
}

// TestInvalidStrategy verifies invalid strategies are rejected.
func TestInvalidStrategy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := manifest.NewStore(".cirrusdata-datasim")
	service := NewService(NewCatalog(), store)

	_, err := service.Init(context.Background(), InitOptions{
		Profile:   "corporate",
		Root:      root,
		TotalSize: "1MiB",
		Seed:      99,
		Strategy:  "unknown",
	})
	if err == nil {
		t.Fatal("expected invalid init strategy error")
	}
}

// TestDefaultWorkerCount verifies the default concurrency floor.
func TestDefaultWorkerCount(t *testing.T) {
	t.Parallel()

	if got := DefaultWorkerCount(); got < 8 {
		t.Fatalf("expected default worker count to be at least 8, got %d", got)
	}
}

// TestInvalidWorkerCount verifies negative worker counts are rejected.
func TestInvalidWorkerCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := manifest.NewStore(".cirrusdata-datasim")
	service := NewService(NewCatalog(), store)

	_, err := service.Init(context.Background(), InitOptions{
		Profile:   "corporate",
		Root:      root,
		TotalSize: "1MiB",
		Seed:      99,
		Strategy:  StrategyBalanced,
		Workers:   -1,
	})
	if err == nil {
		t.Fatal("expected invalid init worker count error")
	}

	_, err = service.Rotate(context.Background(), RotateOptions{
		Root:      root,
		CreatePct: 5,
		DeletePct: 5,
		ModifyPct: 10,
		Seed:      101,
		Strategy:  StrategyBalanced,
		Workers:   -1,
	})
	if err == nil {
		t.Fatal("expected invalid rotate worker count error")
	}
}
