package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cirrusdata/datasim/internal/config"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

// Run executes a command and streams its output to the current process.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Output executes a command and returns its combined output.
func (ExecRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

type Manager struct {
	cfg   config.Config
	state *StateStore
	run   Runner
}

type FormatOptions struct {
	FSType string
	Force  bool
}

type FilesystemRecord struct {
	BlockDevice string    `json:"block_device"`
	MountPoint  string    `json:"mount_point"`
	FSType      string    `json:"fs_type"`
	CreatedAt   time.Time `json:"created_at"`
}

var windowsPhysicalDrivePattern = regexp.MustCompile(`(?i)^\\\\[.?]\\PHYSICALDRIVE(\d+)$`)
var windowsDriveLetterPattern = regexp.MustCompile(`(?i)^([A-Z]):\\?$`)

// NewManager constructs a filesystem lifecycle manager.
func NewManager(cfg config.Config, state *StateStore, runner Runner) *Manager {
	return &Manager{cfg: cfg, state: state, run: runner}
}

// Format formats and mounts a filesystem for datasim use.
func (m *Manager) Format(ctx context.Context, blockDevice string, mountPoint string, opts FormatOptions) (*FilesystemRecord, error) {
	record := &FilesystemRecord{
		BlockDevice: blockDevice,
		MountPoint:  mountPoint,
		FSType:      opts.FSType,
		CreatedAt:   time.Now().UTC(),
	}

	if record.FSType == "" {
		record.FSType = m.cfg.DefaultFSType()
	}

	switch runtime.GOOS {
	case "linux":
		record.MountPoint = filepath.Clean(mountPoint)
		if err := m.prepareLinuxFormat(ctx, record, opts.Force); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(record.MountPoint, 0o755); err != nil {
			return nil, err
		}
		if err := m.createLinux(ctx, record); err != nil {
			return nil, err
		}
	case "windows":
		normalizedMountPoint, err := windowsNormalizeMountPoint(mountPoint)
		if err != nil {
			return nil, err
		}
		record.MountPoint = normalizedMountPoint
		if err := m.prepareWindowsFormat(ctx, record, opts.Force); err != nil {
			return nil, err
		}
		if err := m.createWindows(ctx, record); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("filesystem lifecycle commands are not supported on %s", runtime.GOOS)
	}

	if err := m.state.DeleteByBlockDevice(record.BlockDevice); err != nil {
		return nil, err
	}
	if err := m.state.Put(*record); err != nil {
		return nil, err
	}

	return record, nil
}

// Destroy unmounts and tears down a datasim filesystem.
func (m *Manager) Destroy(ctx context.Context, mountPoint string) error {
	if runtime.GOOS == "windows" {
		normalizedMountPoint, err := windowsNormalizeMountPoint(mountPoint)
		if err != nil {
			return err
		}
		mountPoint = normalizedMountPoint
	} else {
		mountPoint = filepath.Clean(mountPoint)
	}

	record, found, err := m.state.Get(mountPoint)
	if err != nil {
		return err
	}

	if !found && runtime.GOOS == "linux" {
		source, lookupErr := m.run.Output(ctx, "findmnt", "-n", "-o", "SOURCE", "--target", mountPoint)
		if lookupErr == nil && source != "" {
			record = FilesystemRecord{MountPoint: mountPoint, BlockDevice: source}
			found = true
		}
	}

	switch runtime.GOOS {
	case "linux":
		if err := m.destroyLinux(ctx, mountPoint, record, found); err != nil {
			return err
		}
	case "windows":
		if err := m.destroyWindows(ctx, mountPoint, record, found); err != nil {
			return err
		}
	default:
		return fmt.Errorf("filesystem lifecycle commands are not supported on %s", runtime.GOOS)
	}

	return m.state.Delete(mountPoint)
}

// createLinux formats and mounts a Linux filesystem.
func (m *Manager) createLinux(ctx context.Context, record *FilesystemRecord) error {
	mkfs, args, err := linuxMkfsCommand(record.FSType, record.BlockDevice)
	if err != nil {
		return err
	}

	if err := m.run.Run(ctx, mkfs, args...); err != nil {
		return err
	}

	return m.run.Run(ctx, "mount", record.BlockDevice, record.MountPoint)
}

// prepareLinuxFormat validates or clears existing Linux mounts before formatting.
func (m *Manager) prepareLinuxFormat(ctx context.Context, record *FilesystemRecord, force bool) error {
	targetSource, err := m.linuxMountedSource(ctx, record.MountPoint)
	if err != nil {
		return err
	}

	deviceTargets, err := m.linuxMountedTargets(ctx, record.BlockDevice)
	if err != nil {
		return err
	}

	targets := make(map[string]struct{})
	if targetSource != "" {
		if !force {
			return fmt.Errorf("mount point %s is already mounted from %s; use --force to recreate it", record.MountPoint, targetSource)
		}
		targets[record.MountPoint] = struct{}{}
	}
	for _, target := range deviceTargets {
		if !force {
			return fmt.Errorf("block device %s is already mounted at %s; use --force to recreate it", record.BlockDevice, target)
		}
		targets[target] = struct{}{}
	}

	orderedTargets := make([]string, 0, len(targets))
	for target := range targets {
		orderedTargets = append(orderedTargets, target)
	}
	slices.Sort(orderedTargets)

	for _, target := range orderedTargets {
		if err := m.run.Run(ctx, "umount", target); err != nil {
			return err
		}
	}

	return nil
}

// prepareWindowsFormat validates tracked Windows mounts before formatting.
func (m *Manager) prepareWindowsFormat(_ context.Context, record *FilesystemRecord, force bool) error {
	trackedMount, foundMount, err := m.state.Get(record.MountPoint)
	if err != nil {
		return err
	}
	if foundMount && !force {
		return fmt.Errorf("mount point %s is already tracked for %s; use --force to recreate it", record.MountPoint, trackedMount.BlockDevice)
	}

	trackedDevice, foundDevice, err := m.state.GetByBlockDevice(record.BlockDevice)
	if err != nil {
		return err
	}
	if foundDevice && trackedDevice.MountPoint != record.MountPoint && !force {
		return fmt.Errorf("block device %s is already tracked at %s; use --force to recreate it", record.BlockDevice, trackedDevice.MountPoint)
	}

	return nil
}

// destroyLinux unmounts and wipes a Linux filesystem.
func (m *Manager) destroyLinux(ctx context.Context, mountPoint string, record FilesystemRecord, found bool) error {
	if err := m.run.Run(ctx, "umount", mountPoint); err != nil {
		return err
	}

	if !found || record.BlockDevice == "" {
		return fmt.Errorf("no block device information recorded for %s", mountPoint)
	}

	return m.run.Run(ctx, "wipefs", "-a", record.BlockDevice)
}

// linuxMountedSource returns the source device currently mounted at a target path.
func (m *Manager) linuxMountedSource(ctx context.Context, mountPoint string) (string, error) {
	out, err := m.run.Output(ctx, "findmnt", "-rn", "--target", mountPoint, "-o", "SOURCE")
	if err != nil {
		if findmntNotFound(err, out) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(out), nil
}

// linuxMountedTargets returns the mount targets currently using a block device.
func (m *Manager) linuxMountedTargets(ctx context.Context, blockDevice string) ([]string, error) {
	out, err := m.run.Output(ctx, "findmnt", "-rn", "-S", blockDevice, "-o", "TARGET")
	if err != nil {
		if findmntNotFound(err, out) {
			return nil, nil
		}
		return nil, err
	}

	return parseFindmntTargets(out), nil
}

// createWindows formats and mounts a Windows filesystem using a drive letter or directory mount point.
func (m *Manager) createWindows(ctx context.Context, record *FilesystemRecord) error {
	diskNumber, err := windowsDiskNumber(record.BlockDevice)
	if err != nil {
		return err
	}

	createMountPath := ""
	if !windowsIsDriveLetterMountPoint(record.MountPoint) {
		createMountPath = `if (-not (Test-Path $mount)) { New-Item -ItemType Directory -Path $mount -Force | Out-Null }; `
	}

	script := fmt.Sprintf(
		`$ProgressPreference='SilentlyContinue'; `+
			`$ErrorActionPreference='Stop'; `+
			`$mount="%s"; `+
			`$diskNumber=%d; `+
			`$disk=Get-Disk -Number $diskNumber; `+
			`if (-not $disk) { throw "disk not found" }; `+
			`if ($disk.IsOffline) { Set-Disk -Number $diskNumber -IsOffline $false }; `+
			`if ($disk.IsReadOnly) { Set-Disk -Number $diskNumber -IsReadOnly $false }; `+
			`if ($disk.PartitionStyle -eq 'RAW') { Initialize-Disk -Number $diskNumber -PartitionStyle GPT | Out-Null }; `+
			`$partition=Get-Partition -DiskNumber $diskNumber | Where-Object { $_.Type -ne 'Reserved' } | Select-Object -First 1; `+
			`if (-not $partition) { $partition=New-Partition -DiskNumber $diskNumber -UseMaximumSize }; `+
			`%s`+
			`Format-Volume -Partition $partition -FileSystem %s -Confirm:$false -Force | Out-Null; `+
			`Add-PartitionAccessPath -DiskNumber $partition.DiskNumber -PartitionNumber $partition.PartitionNumber -AccessPath $mount`,
		record.MountPoint,
		diskNumber,
		createMountPath,
		strings.ToUpper(record.FSType),
	)
	return m.run.Run(ctx, "powershell", "-NoProfile", "-Command", script)
}

// destroyWindows removes a Windows directory mount point and optionally clears the disk.
func (m *Manager) destroyWindows(ctx context.Context, mountPoint string, record FilesystemRecord, found bool) error {
	script := fmt.Sprintf(
		`$ProgressPreference='SilentlyContinue'; `+
			`$ErrorActionPreference='Stop'; `+
			`$mount="%s"; `+
			`$partition=Get-Partition | Where-Object { $_.AccessPaths -contains $mount } | Select-Object -First 1; `+
			`if (-not $partition) { throw "partition not found" }; `+
			`Remove-PartitionAccessPath -DiskNumber $partition.DiskNumber -PartitionNumber $partition.PartitionNumber -AccessPath $mount; `,
		mountPoint,
	)
	if found {
		diskNumber, err := windowsDiskNumber(record.BlockDevice)
		if err != nil {
			return err
		}
		script += fmt.Sprintf(`Clear-Disk -Number %d -RemoveData -Confirm:$false`, diskNumber)
	}
	return m.run.Run(ctx, "powershell", "-NoProfile", "-Command", script)
}

// windowsDiskNumber normalizes a Windows disk identifier to its numeric disk number.
func windowsDiskNumber(blockDevice string) (int, error) {
	device := strings.TrimSpace(blockDevice)
	if device == "" {
		return 0, fmt.Errorf("block device is required")
	}

	if matches := windowsPhysicalDrivePattern.FindStringSubmatch(device); len(matches) == 2 {
		device = matches[1]
	}

	number, err := strconv.Atoi(device)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("unsupported windows block device %q: use a disk number or \\\\.\\PHYSICALDRIVEn", blockDevice)
	}

	return number, nil
}

// windowsNormalizeMountPoint normalizes a Windows access path for a drive letter or directory mount point.
func windowsNormalizeMountPoint(mountPoint string) (string, error) {
	normalizedMountPoint := strings.TrimSpace(strings.ReplaceAll(mountPoint, "/", `\`))
	if normalizedMountPoint == "" {
		return "", fmt.Errorf("mount point is required")
	}

	if matches := windowsDriveLetterPattern.FindStringSubmatch(normalizedMountPoint); len(matches) == 2 {
		return strings.ToUpper(matches[1]) + `:\`, nil
	}

	return strings.TrimRight(normalizedMountPoint, `\`), nil
}

// parseFindmntTargets parses one mount target per line from findmnt output.
func parseFindmntTargets(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	targets := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		targets = append(targets, line)
	}

	return targets
}

// findmntNotFound reports whether findmnt returned no matching rows.
func findmntNotFound(err error, output string) bool {
	if strings.TrimSpace(output) != "" || err == nil {
		return false
	}

	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// windowsIsDriveLetterMountPoint reports whether the access path is a Windows drive letter.
func windowsIsDriveLetterMountPoint(mountPoint string) bool {
	return windowsDriveLetterPattern.MatchString(strings.TrimSpace(mountPoint))
}

// linuxMkfsCommand returns the mkfs command for a supported Linux filesystem type.
func linuxMkfsCommand(fsType string, blockDevice string) (string, []string, error) {
	switch fsType {
	case "xfs":
		return "mkfs.xfs", []string{"-f", blockDevice}, nil
	case "ext4":
		return "mkfs.ext4", []string{"-F", blockDevice}, nil
	default:
		return "", nil, fmt.Errorf("unsupported linux filesystem type %q", fsType)
	}
}
