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

	// 使用带超时的 context 确保命令不会永久挂起
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = &out
	c.Stderr = &stderr
	
	// log.Printf("[Exec] Running: %v", c.String()) // 调试时可开启
	
	if err := c.Run(); err != nil {
		// 返回带详细 Stderr 的错误信息
		return stderr.String(), fmt.Errorf("%s failed: %w (stderr: %s)", cmd, err, stderr.String())
	}
	return out.String(), nil
}
