package internal

import (
	"os/exec"
)

// NewCommand returns an exec.Cmd that can be used to run external commands.
func NewCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
