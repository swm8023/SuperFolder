//go:build !windows

package superfolder

import (
	"os/exec"
	"runtime"
)

func openPathWithShell(target string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, target)
	configureHiddenCommand(cmd)
	return cmd.Start()
}

func configureHiddenCommand(cmd *exec.Cmd) {}
