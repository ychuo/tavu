package files

import (
	"os"
	"path"
	"runtime"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

var moveAPI struct {
	once   sync.Once
	err    error
	rename func(int32, string, int32, string, uint32) int32
	errno  func() *int32
}

// renameatx_np with RENAME_EXCL atomically refuses an existing destination.
// Anchored parent descriptors keep the syscall within os.Root even when a
// project ancestor changes between validation and the rename.
func moveExclusive(root *os.Root, from, to string) error {
	moveAPI.once.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			moveAPI.err = err
			return
		}
		purego.RegisterLibFunc(&moveAPI.rename, lib, "renameatx_np")
		purego.RegisterLibFunc(&moveAPI.errno, lib, "__error")
	})
	if moveAPI.err != nil {
		return moveAPI.err
	}
	oldParent, err := root.Open(path.Dir(from))
	if err != nil {
		return err
	}
	defer oldParent.Close()
	newParent, err := root.Open(path.Dir(to))
	if err != nil {
		return err
	}
	defer newParent.Close()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if moveAPI.rename(int32(oldParent.Fd()), path.Base(from), int32(newParent.Fd()), path.Base(to), 4) != 0 {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.Errno(*moveAPI.errno())}
	}
	return nil
}
