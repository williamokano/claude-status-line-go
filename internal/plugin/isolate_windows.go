//go:build windows

package plugin

import "os/exec"

// isolate is a no-op on Windows: there's no process-group signal to send, so
// the default Cancel kills the child and WaitDelay bounds how long Output()
// will wait on a pipe a grandchild left open.
func isolate(cmd *exec.Cmd) {}
