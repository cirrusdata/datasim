package filesystem

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cirrusdata/datasim/internal/config"
)

type fakeRunner struct {
	outputs map[string]fakeOutput
	runs    []fakeRun
}

type fakeOutput struct {
	value string
	err   error
}

type fakeRun struct {
	name string
	args []string
}

// Run records a command invocation and returns nil.
func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.runs = append(f.runs, fakeRun{name: name, args: append([]string(nil), args...)})
	return nil
}

// Output returns a configured command result.
func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	result, ok := f.outputs[runnerKey(name, args...)]
	if !ok {
		return "", nil
	}

	return result.value, result.err
}

// runnerKey builds a stable map key for fake runner lookups.
func runnerKey(name string, args ...string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}

// TestWindowsDiskNumberAcceptsDiskNumbers verifies supported Windows disk identifiers.
func TestWindowsDiskNumberAcceptsDiskNumbers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		blockDevice string
		want        int
	}{
		{name: "numeric", blockDevice: "3", want: 3},
		{name: "physical drive path", blockDevice: `\\.\PHYSICALDRIVE3`, want: 3},
		{name: "physical drive path case insensitive", blockDevice: `\\.\physicaldrive12`, want: 12},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := windowsDiskNumber(tc.blockDevice)
			if err != nil {
				t.Fatalf("windowsDiskNumber(%q) returned error: %v", tc.blockDevice, err)
			}

			if got != tc.want {
				t.Fatalf("windowsDiskNumber(%q) = %d, want %d", tc.blockDevice, got, tc.want)
			}
		})
	}
}

// TestWindowsDiskNumberRejectsUnsupportedValues verifies unsupported Windows disk identifiers are rejected.
func TestWindowsDiskNumberRejectsUnsupportedValues(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"",
		"/dev/sdc",
		`\\.\PHYSICALDRIVE`,
		"disk3",
		"-1",
	}

	for _, blockDevice := range testCases {
		blockDevice := blockDevice
		t.Run(blockDevice, func(t *testing.T) {
			t.Parallel()

			_, err := windowsDiskNumber(blockDevice)
			if err == nil {
				t.Fatalf("windowsDiskNumber(%q) returned nil error", blockDevice)
			}

			if !strings.Contains(err.Error(), "unsupported windows block device") && blockDevice != "" {
				t.Fatalf("windowsDiskNumber(%q) error %q did not mention unsupported windows block device", blockDevice, err)
			}
		})
	}
}

// TestWindowsNormalizeMountPointAcceptsDriveLetters verifies Windows drive-letter access paths normalize correctly.
func TestWindowsNormalizeMountPointAcceptsDriveLetters(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		mountPoint string
		want       string
	}{
		{name: "uppercase drive with slash", mountPoint: `X:\`, want: `X:\`},
		{name: "uppercase drive without slash", mountPoint: `X:`, want: `X:\`},
		{name: "lowercase drive", mountPoint: `x:\`, want: `X:\`},
		{name: "directory mount", mountPoint: `C:\datasim\mount\`, want: `C:\datasim\mount`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := windowsNormalizeMountPoint(tc.mountPoint)
			if err != nil {
				t.Fatalf("windowsNormalizeMountPoint(%q) returned error: %v", tc.mountPoint, err)
			}

			if got != tc.want {
				t.Fatalf("windowsNormalizeMountPoint(%q) = %q, want %q", tc.mountPoint, got, tc.want)
			}
		})
	}
}

// TestWindowsNormalizeMountPointRejectsEmpty verifies an empty mount point is rejected.
func TestWindowsNormalizeMountPointRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := windowsNormalizeMountPoint("")
	if err == nil {
		t.Fatal("windowsNormalizeMountPoint(\"\") returned nil error")
	}
}

// TestPrepareLinuxFormatRejectsMountedTargetWithoutForce verifies format refuses a mounted target unless forced.
func TestPrepareLinuxFormatRejectsMountedTargetWithoutForce(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			runnerKey("findmnt", "-rn", "--target", "/mnt/datasim", "-o", "SOURCE"): {value: "/dev/sdb1"},
			runnerKey("findmnt", "-rn", "-S", "/dev/sdc1", "-o", "TARGET"):          {},
		},
	}
	manager := &Manager{run: runner}

	err := manager.prepareLinuxFormat(context.Background(), &FilesystemRecord{
		BlockDevice: "/dev/sdc1",
		MountPoint:  "/mnt/datasim",
	}, false)
	if err == nil {
		t.Fatal("prepareLinuxFormat returned nil error")
	}
	if !strings.Contains(err.Error(), "already mounted") {
		t.Fatalf("prepareLinuxFormat error %q did not mention an existing mount", err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("prepareLinuxFormat ran commands unexpectedly: %+v", runner.runs)
	}
}

// TestPrepareLinuxFormatUnmountsMountedTargetsWithForce verifies force mode unmounts current device targets.
func TestPrepareLinuxFormatUnmountsMountedTargetsWithForce(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			runnerKey("findmnt", "-rn", "--target", "/mnt/new", "-o", "SOURCE"): {},
			runnerKey("findmnt", "-rn", "-S", "/dev/sdc1", "-o", "TARGET"):      {value: "/mnt/new\n/mnt/old"},
		},
	}
	manager := &Manager{run: runner}

	if err := manager.prepareLinuxFormat(context.Background(), &FilesystemRecord{
		BlockDevice: "/dev/sdc1",
		MountPoint:  "/mnt/new",
	}, true); err != nil {
		t.Fatalf("prepareLinuxFormat returned error: %v", err)
	}

	got := make([]fakeRun, len(runner.runs))
	copy(got, runner.runs)
	want := []fakeRun{
		{name: "umount", args: []string{"/mnt/new"}},
		{name: "umount", args: []string{"/mnt/old"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prepareLinuxFormat runs = %+v, want %+v", got, want)
	}
}

// TestStateStoreDeleteByBlockDevice verifies stale block-device records can be removed before rewriting state.
func TestStateStoreDeleteByBlockDevice(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewStateStore(config.Config{StateFile: filepath.Join(dir, "state.json")})
	if err != nil {
		t.Fatalf("NewStateStore returned error: %v", err)
	}

	records := []FilesystemRecord{
		{BlockDevice: "/dev/sdc1", MountPoint: "/mnt/one"},
		{BlockDevice: "/dev/sdd1", MountPoint: "/mnt/two"},
	}
	for _, record := range records {
		if err := store.Put(record); err != nil {
			t.Fatalf("Put(%+v) returned error: %v", record, err)
		}
	}

	if err := store.DeleteByBlockDevice("/dev/sdc1"); err != nil {
		t.Fatalf("DeleteByBlockDevice returned error: %v", err)
	}

	if _, found, err := store.GetByBlockDevice("/dev/sdc1"); err != nil {
		t.Fatalf("GetByBlockDevice returned error: %v", err)
	} else if found {
		t.Fatal("GetByBlockDevice unexpectedly found deleted record")
	}

	record, found, err := store.Get("/mnt/two")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !found {
		t.Fatal("Get did not find remaining record")
	}
	if record.BlockDevice != "/dev/sdd1" {
		t.Fatalf("remaining record block device = %q, want /dev/sdd1", record.BlockDevice)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if strings.Contains(string(data), "/dev/sdc1") {
		t.Fatalf("state file still contained deleted block device: %s", string(data))
	}
}

// TestFindmntNotFoundRecognizesEmptyExitStatus verifies empty non-match exit status is treated as no mount.
func TestFindmntNotFoundRecognizesEmptyExitStatus(t *testing.T) {
	t.Parallel()

	if !findmntNotFound(&exec.ExitError{}, "") {
		t.Fatal("findmntNotFound did not recognize empty exit error")
	}
	if findmntNotFound(nil, "") {
		t.Fatal("findmntNotFound treated nil error as no match")
	}
}
