//go:build !windows

package steam

import (
	"os/exec"
	"syscall"
	"time"
)

// configureCmdProcessGroup places SteamCMD in its own process group so context cancellation
// terminates both steamcmd.sh and its child 32-bit steamcmd binary cleanly.
func configureCmdProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}
