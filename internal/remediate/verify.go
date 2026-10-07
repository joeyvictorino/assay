package remediate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Recheck re-runs a checker against a rebuilt lab and reports whether the
// weakness is gone. It must make its requests through internal/httpx.
type Recheck func(baseURL string) (fixed bool, err error)

// maxCopyFile caps the size of any single file copied into the temp lab.
const maxCopyFile = 32 << 20

// Verify copies labDir into a temporary directory, applies diff, builds the
// lab's command with the Go toolchain, starts it on a free loopback port,
// calls recheck and stops it. The temporary directory is always removed.
//
// The lab must be a Go module whose main package is the single directory
// under cmd/ (or the module root when cmd/ is absent) and must accept
// "-addr host:port".
func Verify(ctx context.Context, labDir, diff string, recheck Recheck) (bool, error) {
	if recheck == nil {
		return false, errors.New("remediate: recheck is required")
	}
	if strings.TrimSpace(diff) == "" {
		return false, errors.New("remediate: empty diff")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return false, fmt.Errorf("remediate: go toolchain not found: %w", err)
	}
	tmp, err := os.MkdirTemp("", "assay-remediate-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)

	if err := copyDir(labDir, tmp); err != nil {
		return false, fmt.Errorf("remediate: copy lab: %w", err)
	}
	if err := ApplyUnified(tmp, diff); err != nil {
		return false, fmt.Errorf("remediate: apply: %w", err)
	}
	pkg, err := mainPackage(tmp)
	if err != nil {
		return false, err
	}
	bin := filepath.Join(tmp, ".assay-bin", "lab")
	build := exec.CommandContext(ctx, goBin, "build", "-o", bin, pkg)
	build.Dir = tmp
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		return false, fmt.Errorf("remediate: build failed: %v: %s", err, truncate(string(out), 2000))
	}

	port, err := freePort()
	if err != nil {
		return false, err
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, bin, "-addr", addr)
	cmd.Dir = tmp
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("remediate: start lab: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	if err := waitForPort(ctx, addr, 30*time.Second); err != nil {
		return false, err
	}
	return recheck("http://" + addr)
}

// mainPackage picks the command to build: the sole directory under cmd/, or
// the module root.
func mainPackage(root string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		if _, err := os.Stat(filepath.Join(root, "main.go")); err == nil {
			return ".", nil
		}
		return "", errors.New("remediate: lab has neither cmd/ nor main.go")
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", errors.New("remediate: cmd/ has no packages")
	}
	sort.Strings(dirs)
	return "./cmd/" + dirs[0], nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(ctx context.Context, addr string, max time.Duration) error {
	deadline := time.Now().Add(max)
	for {
		c, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("remediate: lab did not listen on %s within %s", addr, max)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// copyDir copies src into dst, skipping VCS metadata, build outputs and
// oversized files. Symlinks are not followed.
func copyDir(src, dst string) error {
	src = filepath.Clean(src)
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", ".assay-bin":
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxCopyFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, info.Mode().Perm()|0o600)
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
