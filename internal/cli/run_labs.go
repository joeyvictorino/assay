package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/labs/dvwa"
	"github.com/joeyvictorino/assay/internal/labs/juiceshop"
	"github.com/joeyvictorino/assay/internal/labs/syntheticops"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/overlap"
)

// defaultLabURLs are the loopback ports the CI workflow exposes.
var defaultLabURLs = map[string]string{
	syntheticops.Name: "http://127.0.0.1:9000",
	juiceshop.Name:    "http://127.0.0.1:3000",
	dvwa.Name:         "http://127.0.0.1:8080",
}

// parseLabURLs turns repeated "name=url" flags into a map over the defaults.
func parseLabURLs(flags []string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range defaultLabURLs {
		out[k] = v
	}
	for _, f := range flags {
		name, url, ok := strings.Cut(f, "=")
		if !ok || name == "" || url == "" {
			return nil, fmt.Errorf("--lab-url %q: want name=url", f)
		}
		out[name] = strings.TrimRight(url, "/")
	}
	return out, nil
}

// newLab builds the adapter for a named lab. labDir is the checkout root
// of labs/ (ground truth and reference lists live there).
func newLab(name, baseURL, labDir string, client *httpx.Client) (labs.Lab, error) {
	switch name {
	case syntheticops.Name:
		return syntheticops.New(baseURL, filepath.Join(labDir, "synthetic-ops"), client), nil
	case juiceshop.Name:
		return juiceshop.New(baseURL, filepath.Join(labDir, "juice-shop", "accounts.yaml"), filepath.Join(labDir, "juice-shop", "reference_findings.json"), client), nil
	case dvwa.Name:
		return dvwa.New(baseURL, filepath.Join(labDir, "dvwa", "reference_findings.json"), client), nil
	}
	return nil, fmt.Errorf("unknown lab %q (known: synthetic-ops, juice-shop, dvwa)", name)
}

// groundTruthFor converts a lab's ground truth into the overlap package's
// shape. ok is false when the list is not exhaustive (no precision claim).
func groundTruthFor(lab labs.Lab) ([]overlap.GroundTruthEntry, bool) {
	entries, exhaustive := lab.GroundTruth()
	out := make([]overlap.GroundTruthEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, overlap.GroundTruthEntry{
			Lab: e.Lab, Class: string(e.Class), Method: e.Method,
			PathTemplate: e.PathTemplate, Param: e.Param,
		})
	}
	return out, exhaustive
}

// gtLookup indexes ground truth by (class, path template) for remediation.
func gtLookup(lab labs.Lab) func(f model.Finding) *labs.GroundTruthEntry {
	entries, _ := lab.GroundTruth()
	return func(f model.Finding) *labs.GroundTruthEntry {
		return labs.FindByClass(entries, f.Class, f.Location.PathTemplate)
	}
}

// labHealthy reports whether the lab answers; the run records unhealthy
// labs instead of failing the whole run.
func labHealthy(ctx context.Context, lab labs.Lab) error {
	if err := lab.Health(ctx); err != nil {
		return err
	}
	return lab.Setup(ctx)
}
