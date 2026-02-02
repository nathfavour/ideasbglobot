package internal

import (
	"io"
	"os"
)

// SafeMove handles moving files across partitions by copying and deleting if rename fails
func SafeMove(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Fallback to copy+delete
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	source.Close()
	return os.Remove(src)
}
