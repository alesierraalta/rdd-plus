package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BackupSummary is one complete backup in the store, as ListBackups reports it.
type BackupSummary struct {
	ID        string
	CreatedAt string
	FileCount int
}

// RestoreAction is one planned restore step; planning writes nothing.
type RestoreAction struct {
	OriginalPath string
	SnapshotPath string
	Mode         uint32
	// data is the snapshot bytes read while planning, so DryRun proves the backup is complete
	// and apply never discovers a missing snapshot after it already overwrote an earlier file.
	data []byte
}

// RestoreReport says what a restore did, or would do under DryRun.
type RestoreReport struct {
	ID       string
	DryRun   bool
	Actions  []RestoreAction
	Restored []string
}

// ListBackups answers every complete backup in the store, newest first: ids are timestamps, so
// descending id order is creation order. A directory without a manifest is a writer that has not
// finished and is skipped; a manifest that exists but cannot be read fails the whole listing,
// because a store that cannot be read must not be silently partially listed.
func ListBackups() ([]BackupSummary, error) {
	store, err := newBackupStore()
	if err != nil {
		return nil, err
	}
	return listBackups(store.root)
}

// Restore copies one backup store entry back onto the original paths its manifest recorded,
// preserving each file's mode; an empty id selects the latest backup. The manifest is the
// authority for both ends of every copy: snapshot bytes must resolve inside the backup
// directory and original paths must be absolute as recorded, so a hand-edited manifest cannot
// aim the write anywhere the store itself could not already name. DryRun plans and reads but
// writes nothing.
func Restore(id string, dryRun bool) (RestoreReport, error) {
	report := RestoreReport{DryRun: dryRun}
	store, err := newBackupStore()
	if err != nil {
		return report, err
	}
	backupsRoot := filepath.Join(store.root, "backups")
	backups, err := listBackups(store.root)
	if err != nil {
		return report, err
	}
	if len(backups) == 0 {
		return report, fmt.Errorf("no backups found under %s", backupsRoot)
	}
	selected, err := selectBackup(backups, id)
	if err != nil {
		return report, err
	}
	report.ID = selected.ID
	backupDir := filepath.Join(backupsRoot, selected.ID)
	manifest, err := readBackupManifest(backupDir)
	if err != nil {
		return report, err
	}
	for _, entry := range manifest.Entries {
		action, err := planRestoreEntry(backupDir, entry)
		if err != nil {
			return report, err
		}
		report.Actions = append(report.Actions, action)
	}
	if dryRun {
		return report, nil
	}
	for _, action := range report.Actions {
		if err := applyRestore(action); err != nil {
			return report, err
		}
		report.Restored = append(report.Restored, action.OriginalPath)
	}
	return report, nil
}

// selectBackup picks the newest backup or the named one. The id is matched against the listed
// directory entries and only a matched entry's name is ever joined into a path, so a crafted
// --id cannot leave the store.
func selectBackup(available []BackupSummary, id string) (BackupSummary, error) {
	if id == "" {
		return available[0], nil
	}
	for _, candidate := range available {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	ids := make([]string, 0, len(available))
	for _, candidate := range available {
		ids = append(ids, candidate.ID)
	}
	return BackupSummary{}, fmt.Errorf("unknown backup id %q; available ids: %s", id, strings.Join(ids, ", "))
}

// listBackups scans <root>/backups and answers newest first, skipping directories that do not
// hold a manifest yet.
func listBackups(root string) ([]BackupSummary, error) {
	backupsRoot := filepath.Join(root, "backups")
	entries, err := os.ReadDir(backupsRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read backup store %s: %w", backupsRoot, err)
	}
	summaries := make([]BackupSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		backupDir := filepath.Join(backupsRoot, entry.Name())
		manifest, err := readBackupManifest(backupDir)
		if os.IsNotExist(err) {
			continue // ensure() creates the directory before the manifest; an unfinished one is not a backup
		}
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, BackupSummary{
			ID:        entry.Name(),
			CreatedAt: manifest.CreatedAt,
			FileCount: len(manifest.Entries),
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].ID > summaries[j].ID })
	return summaries, nil
}

func readBackupManifest(backupDir string) (backupManifest, error) {
	var manifest backupManifest
	path := filepath.Join(backupDir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, err
		}
		return manifest, fmt.Errorf("read backup manifest %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("backup manifest %s is corrupt: %w", path, err)
	}
	return manifest, nil
}

// planRestoreEntry validates one manifest entry and reads its snapshot bytes. Every refusal
// happens here, before anything is written, so a bad entry stops the whole restore the way an
// uninstall refusal stops the run before the first deletion.
func planRestoreEntry(backupDir string, entry backupEntry) (RestoreAction, error) {
	if !filepath.IsAbs(entry.OriginalPath) {
		return RestoreAction{}, fmt.Errorf("refuse restore: manifest original path %q is not absolute", entry.OriginalPath)
	}
	snapshot := filepath.Join(backupDir, filepath.FromSlash(entry.SnapshotPath))
	if !withinDir(backupDir, snapshot) {
		return RestoreAction{}, fmt.Errorf("refuse restore: snapshot path %q escapes backup dir %s", entry.SnapshotPath, backupDir)
	}
	info, err := os.Stat(snapshot)
	if err != nil {
		return RestoreAction{}, fmt.Errorf("read backup snapshot %s: %w", snapshot, err)
	}
	if !info.Mode().IsRegular() {
		return RestoreAction{}, fmt.Errorf("read backup snapshot %s: not a regular file", snapshot)
	}
	data, err := os.ReadFile(snapshot)
	if err != nil {
		return RestoreAction{}, fmt.Errorf("read backup snapshot %s: %w", snapshot, err)
	}
	mode := entry.Mode
	if mode == 0 {
		mode = uint32(info.Mode().Perm())
	}
	return RestoreAction{
		OriginalPath: filepath.Clean(entry.OriginalPath),
		SnapshotPath: snapshot,
		Mode:         mode,
		data:         data,
	}, nil
}

// withinDir reports whether target lives inside base after both are made absolute and cleaned,
// which is what stops a manifest snapshot path containing .. from reading a file outside the
// backup directory.
func withinDir(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	return absTarget != absBase && strings.HasPrefix(absTarget, absBase+string(filepath.Separator))
}

func applyRestore(action RestoreAction) error {
	if err := os.MkdirAll(filepath.Dir(action.OriginalPath), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", action.OriginalPath, err)
	}
	if err := os.WriteFile(action.OriginalPath, action.data, os.FileMode(action.Mode).Perm()); err != nil {
		return fmt.Errorf("restore %s: %w", action.OriginalPath, err)
	}
	// WriteFile applies the mode only when it creates the file, so an overwrite needs the chmod
	// to land the mode the manifest recorded.
	if err := os.Chmod(action.OriginalPath, os.FileMode(action.Mode).Perm()); err != nil {
		return fmt.Errorf("preserve restored mode %s: %w", action.OriginalPath, err)
	}
	return nil
}

// String renders the report for a terminal: a dry run shows the plan, an applied run shows what
// was written back.
func (r RestoreReport) String() string {
	prefix := ""
	if r.DryRun {
		prefix = "[dry-run] "
	}
	var b strings.Builder
	switch {
	case r.DryRun && len(r.Actions) == 0:
		return prefix + "nothing to restore\n"
	case r.DryRun:
		for _, action := range r.Actions {
			fmt.Fprintf(&b, "%s[restore] %s <- %s (mode %#o)\n", prefix, action.OriginalPath, action.SnapshotPath, action.Mode)
		}
	case len(r.Restored) == 0:
		return "nothing to restore\n"
	default:
		for _, path := range r.Restored {
			fmt.Fprintf(&b, "restored %s\n", path)
		}
	}
	return b.String()
}
