package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkTreePolicy checks every existing entry before a recursive operation
// mutates any of them. WalkDir does not follow symlinks. Missing destinations
// are checked by name; transfers also check their incoming paths separately.
func checkTreePolicy(ctx context.Context, root string, intent PathIntent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := CheckPathPolicy(ctx, root, intent); err != nil {
		return err
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect directory operation: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return CheckPathPolicy(ctx, path, intent)
	})
}

// checkTransferPolicy preflights source access, destination replacement, and
// every incoming destination path, including paths that do not exist yet.
func checkTransferPolicy(ctx context.Context, source, destination string, sourceIntent PathIntent) error {
	if err := checkTreePolicy(ctx, source, sourceIntent); err != nil {
		return err
	}
	if err := checkTreePolicy(ctx, destination, IntentMutate); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// copyPath refuses symlinks; reject them before replacing the destination.
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("entry %q is a symlink; refusing transfer", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		return CheckPathPolicy(ctx, filepath.Join(destination, rel), IntentMutate)
	})
}
