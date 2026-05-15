//go:build windows

package superfolder

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	createNoWindow = 0x08000000
	swShowNormal   = 1
)

var procShellExecuteW = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

func openPathWithShell(target string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	path, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(path)),
		0,
		0,
		swShowNormal,
	)
	if ret > 32 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return fmt.Errorf("ShellExecuteW failed with code %d", ret)
}

func configureHiddenCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
