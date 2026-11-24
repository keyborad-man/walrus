//go:build windows

package steam

import (
	"os/exec"
	"time"
)

func configureCmdProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = 5 * time.Second
}
