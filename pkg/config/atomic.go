package config

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path via temp-then-rename in the same directory.
// Used by Config.Write and by pkg/item.Store.Save (AWIT-0ND56E3G).
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		// Windows cannot rename over an existing file (guide §1).
		if rmErr := os.Remove(path); rmErr == nil {
			err = os.Rename(name, path)
		}
		if err != nil {
			return err
		}
	}
	ok = true
	return nil
}
