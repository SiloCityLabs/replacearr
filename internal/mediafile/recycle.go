package mediafile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResolveRecycleBin returns the configured recycle bin path, or configDir/RecycleBin when blank.
func ResolveRecycleBin(configured, configDir string) string {
	bin := strings.TrimSpace(configured)
	if bin != "" {
		return filepath.Clean(bin)
	}
	dir := strings.TrimSpace(configDir)
	if dir == "" {
		dir = "./config"
	}
	return filepath.Join(filepath.Clean(dir), "RecycleBin")
}

// ChangeRecycleBinPath empties and removes the old recycle bin directory when the
// effective path changes, then ensures the new directory exists. This prevents
// orphaned bins after a settings change.
func ChangeRecycleBinPath(oldConfigured, newConfigured, configDir string, protectedRoots []string) error {
	oldBin := ResolveRecycleBin(oldConfigured, configDir)
	newBin := ResolveRecycleBin(newConfigured, configDir)
	if oldBin == newBin {
		return nil
	}
	if err := EmptyRecycleBin(oldBin, configDir, protectedRoots); err != nil {
		return fmt.Errorf("empty previous recycle bin %q: %w", oldBin, err)
	}
	if err := os.MkdirAll(newBin, 0o755); err != nil {
		return fmt.Errorf("create recycle bin %q: %w", newBin, err)
	}
	return nil
}

// EmptyRecycleBin permanently deletes the recycle bin directory after safety checks.
// Missing paths are a no-op.
func EmptyRecycleBin(bin, configDir string, protectedRoots []string) error {
	bin = filepath.Clean(strings.TrimSpace(bin))
	if err := assertSafeRecycleBinPath(bin, configDir, protectedRoots); err != nil {
		return err
	}
	if _, err := os.Stat(bin); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(bin)
}

func assertSafeRecycleBinPath(bin, configDir string, protectedRoots []string) error {
	bin = filepath.Clean(strings.TrimSpace(bin))
	if bin == "" || bin == "." || bin == string(os.PathSeparator) {
		return fmt.Errorf("refusing to empty unsafe recycle bin path %q", bin)
	}
	if pathDepth(bin) < 2 {
		return fmt.Errorf("refusing to empty shallow recycle bin path %q", bin)
	}
	cfgDir := filepath.Clean(strings.TrimSpace(configDir))
	if cfgDir == "" {
		cfgDir = "./config"
	}
	if bin == cfgDir {
		return fmt.Errorf("refusing to empty config directory %q", bin)
	}
	sep := string(os.PathSeparator)
	for _, root := range protectedRoots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" {
			continue
		}
		if bin == root || strings.HasPrefix(root, bin+sep) {
			return fmt.Errorf("refusing to empty recycle bin path %q that covers root folder %q", bin, root)
		}
	}
	return nil
}

func pathDepth(p string) int {
	p = filepath.Clean(p)
	if p == "" || p == "." || p == string(os.PathSeparator) {
		return 0
	}
	vol := filepath.VolumeName(p)
	rest := strings.TrimPrefix(p, vol)
	rest = strings.Trim(rest, `/\`)
	if rest == "" {
		return 0
	}
	n := 0
	for _, part := range strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "" || part == "." {
			continue
		}
		n++
	}
	return n
}

// ValidateUnderRoot ensures path is a non-empty path strictly inside root.
func ValidateUnderRoot(path, root string) (cleanPath, cleanRoot string, err error) {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" {
		return "", "", fmt.Errorf("refusing to delete: empty path")
	}
	if root == "" {
		return "", "", fmt.Errorf("refusing to delete files without a root folder")
	}
	cleanPath = filepath.Clean(path)
	cleanRoot = filepath.Clean(root)
	sep := string(os.PathSeparator)
	if cleanPath == cleanRoot || !strings.HasPrefix(cleanPath, cleanRoot+sep) {
		return "", "", fmt.Errorf("path %q is outside root folder %q", cleanPath, cleanRoot)
	}
	return cleanPath, cleanRoot, nil
}

// DeleteUnderRoot moves path into recycleBin when it lives under root.
// If the path does not exist, it is a no-op.
func DeleteUnderRoot(path, root, recycleBin string) error {
	cleanPath, _, err := ValidateUnderRoot(path, root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return MoveToRecycleBin(cleanPath, recycleBin)
}

// MoveToRecycleBin relocates src into recycleBin using a unique destination name.
// Prefers rename; falls back to recursive copy then remove on cross-device failure.
func MoveToRecycleBin(src, recycleBin string) error {
	src = filepath.Clean(strings.TrimSpace(src))
	recycleBin = filepath.Clean(strings.TrimSpace(recycleBin))
	if src == "" {
		return fmt.Errorf("empty source path")
	}
	if recycleBin == "" {
		return fmt.Errorf("recycle bin path is required")
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(recycleBin, 0o755); err != nil {
		return fmt.Errorf("create recycle bin: %w", err)
	}
	base := filepath.Base(src)
	dest := filepath.Join(recycleBin, fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), base))
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyPath(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("move to recycle bin: %w", err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("remove source after recycle copy: %w", err)
	}
	return nil
}

func copyPath(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest, info.Mode())
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := copyDir(from, to); err != nil {
				return err
			}
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(from, to, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
