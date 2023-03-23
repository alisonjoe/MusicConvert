package util

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

func Run(ctx context.Context, cmd string, args []string) (string, error) {
	var out bytes.Buffer
	var stderr bytes.Buffer

	c := exec.Command(cmd, args...)
	c.Stdout = &out
	c.Stderr = &stderr
	fmt.Printf("%v\n", c.String())
	if err := c.Run(); err != nil {
		return stderr.String(), err
	}
	return out.String(), nil
}
