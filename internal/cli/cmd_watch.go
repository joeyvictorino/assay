package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

func init() {
	Register("watch", "monitor policy and manifests for changes, re-lint on change", runWatch)
}

func snapshotFiles(paths []string) map[string]int64 {
	m := make(map[string]int64, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			m[p] = -1
			continue
		}
		m[p] = info.ModTime().UnixNano()
	}
	return m
}

func filesChanged(before, after map[string]int64) bool {
	for k, v := range after {
		if bv, ok := before[k]; !ok || bv != v {
			return true
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			return true
		}
	}
	return false
}

func collectWatchPaths(policyPath, manifestDir string) ([]string, error) {
	paths := []string{policyPath}
	entries, err := os.ReadDir(manifestDir)
	if err != nil {
		return nil, fmt.Errorf("read manifests dir: %w", err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		paths = append(paths, filepath.Join(manifestDir, e.Name()))
	}
	return paths, nil
}

type watchEvent struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`
	Issues    int    `json:"issues,omitempty"`
}

func runWatch(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay watch", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	interval := fs.Float64("interval", 2, "polling interval in seconds")
	maxCycles := fs.Int("max-cycles", 0, "stop after N cycles (0 = unlimited, for testing)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay watch: --policy and --manifests are required")
		return ExitError
	}

	paths, err := collectWatchPaths(*policyPath, *manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay watch: %v\n", err)
		return ExitError
	}

	prev := snapshotFiles(paths)

	emit := func(ev watchEvent) {
		switch *format {
		case "json":
			enc := json.NewEncoder(stdout)
			_ = enc.Encode(ev)
		default:
			if ev.Issues > 0 {
				fmt.Fprintf(stdout, "[%s] %s (%d issues)\n", ev.Timestamp, ev.Event, ev.Issues)
			} else {
				fmt.Fprintf(stdout, "[%s] %s\n", ev.Timestamp, ev.Event)
			}
		}
	}

	emit(watchEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     "watching",
	})

	dur := time.Duration(*interval * float64(time.Second))
	cycle := 0
	for {
		if *maxCycles > 0 {
			cycle++
			if cycle > *maxCycles {
				break
			}
		}
		time.Sleep(dur)

		paths, err = collectWatchPaths(*policyPath, *manifests)
		if err != nil {
			emit(watchEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Event:     "error: " + err.Error(),
			})
			continue
		}

		cur := snapshotFiles(paths)
		if !filesChanged(prev, cur) {
			continue
		}
		prev = cur

		eng, err := policy.Load(*policyPath)
		if err != nil {
			emit(watchEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Event:     "policy load error: " + err.Error(),
			})
			continue
		}

		files, err := loadManifestDir(*manifests)
		if err != nil {
			emit(watchEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Event:     "manifest load error: " + err.Error(),
			})
			continue
		}
		var ms []model.ToolManifest
		for _, f := range files {
			ms = append(ms, f.M)
		}
		issues := policy.Lint(eng, ms)
		emit(watchEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Event:     "change detected, re-linted",
			Issues:    len(issues),
		})
	}
	return ExitPass
}
