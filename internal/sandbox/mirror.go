package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ca7ai/glasswall/internal/db"
)

var excludedPaths = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"dist":         true,
	".DS_Store":    true,
	".glasswall":   true,
}

// CreateMirror recursively copies sourceDir to a temp dir /private/tmp/glasswall-runs/<runID>.
func CreateMirror(sourceDir string, runID string) (string, error) {
	tempDir := filepath.Join("/private/tmp", "glasswall-runs", runID)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return "", err
	}

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		// Check if any path segment is excluded
		parts := strings.Split(rel, string(filepath.Separator))
		for _, part := range parts {
			if excludedPaths[part] {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		targetPath := filepath.Join(tempDir, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath, info)
	})

	if err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}

func copyFile(src, dst string, info os.FileInfo) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// fileHash computes the SHA256 of a file.
func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// DiffMirror compares sourceDir and mirrorDir to discover created, modified, and deleted files.
func DiffMirror(sourceDir string, mirrorDir string) (db.FileChanges, error) {
	var changes db.FileChanges
	sourceHashes := make(map[string]string)
	mirrorHashes := make(map[string]string)

	// Collect source files & hashes
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		parts := strings.Split(rel, string(filepath.Separator))
		for _, part := range parts {
			if excludedPaths[part] {
				return nil
			}
		}

		hash, err := fileHash(path)
		if err == nil {
			sourceHashes[rel] = hash
		}
		return nil
	})
	if err != nil {
		return changes, err
	}

	// Collect mirror files & hashes
	err = filepath.Walk(mirrorDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(mirrorDir, path)
		if err != nil {
			return err
		}

		hash, err := fileHash(path)
		if err == nil {
			mirrorHashes[rel] = hash
		}
		return nil
	})
	if err != nil {
		return changes, err
	}

	// Analyze changes
	for rel, mirrorHash := range mirrorHashes {
		sourceHash, exists := sourceHashes[rel]
		if !exists {
			changes.Created = append(changes.Created, rel)
		} else if sourceHash != mirrorHash {
			changes.Modified = append(changes.Modified, rel)
		}
	}

	for rel := range sourceHashes {
		if _, exists := mirrorHashes[rel]; !exists {
			changes.Deleted = append(changes.Deleted, rel)
		}
	}

	return changes, nil
}

// CleanupMirror removes a mirror workspace directory.
func CleanupMirror(mirrorDir string) error {
	if !strings.HasPrefix(mirrorDir, "/private/tmp/glasswall-runs") {
		return os.ErrPermission // Safety check to prevent accidental broad deletes
	}
	return os.RemoveAll(mirrorDir)
}
