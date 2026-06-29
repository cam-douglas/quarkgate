package registry

import (
	"bytes"
	"fmt"
	"os/exec"
)

func realExecCommand(name string, args []string, dir string, stdin []byte) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %s", err, string(ee.Stderr))
		}
		return nil, err
	}
	return out, nil
}
