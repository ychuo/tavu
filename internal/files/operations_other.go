//go:build !darwin

package files

import (
	"fmt"
	"os"
)

func moveExclusive(root *os.Root, from, to string) error {
	info, err := root.Lstat(from)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("Folder moves require the macOS app")
	}
	if err = root.Link(from, to); err != nil {
		return err
	}
	return root.Remove(from)
}
