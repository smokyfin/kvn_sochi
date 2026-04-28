package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Process struct {
	name string
	cmd  *exec.Cmd
	mu   sync.Mutex
	done chan error
}

func Start(ctx context.Context, name string, binary string, args []string, env []string, privateDir string) (*Process, error) {
	if binary == "" {
		return nil, fmt.Errorf("%s binary path is empty", name)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = privateDir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	p := &Process{name: name, cmd: cmd, done: make(chan error, 1)}
	go func() {
		p.done <- cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

func (p *Process) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	select {
	case err := <-p.done:
		return ignoreExpectedExit(err)
	default:
	}

	pid := p.cmd.Process.Pid
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}

	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()

	select {
	case err := <-p.done:
		return ignoreExpectedExit(err)
	case <-timer.C:
		slog.Warn("process did not stop after SIGTERM, killing", "name", p.name)
		if pid > 0 {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	case <-ctx.Done():
		if pid > 0 {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
		return ctx.Err()
	}

	select {
	case err := <-p.done:
		return ignoreExpectedExit(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ignoreExpectedExit(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}
