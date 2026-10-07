// Package zdr makes zero data retention a tested property.
//
// The only production TranscriptSink discards everything. ScanTree lets a
// test (or an operator) prove that a canary planted in a model transcript
// never reached disk, by scanning every file under a root byte for byte.
// Ciphertext is scanned as bytes too; the canary is random, so an encrypted
// audit log cannot contain it by accident either.
package zdr

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

// NullSink is the production transcript sink: it keeps nothing.
type NullSink struct{}

// Write implements model.TranscriptSink and discards body.
func (NullSink) Write(_ context.Context, _, _ string, _ []byte) error { return nil }

var _ model.TranscriptSink = NullSink{}

// CanaryPrefix starts every canary produced by NewCanary.
const CanaryPrefix = "ZDR-CANARY-"

// NewCanary returns a fresh random marker of the form ZDR-CANARY-<16 hex>.
// Planted in a prompt, it must never be found by ScanTree afterwards.
func NewCanary() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("zdr: " + err.Error())
	}
	return CanaryPrefix + hex.EncodeToString(b)
}

// Hit is one occurrence of a needle in a file.
type Hit struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Needle string `json:"needle"`
}

// ScanTree walks every regular file under root and reports each occurrence
// of each needle with its byte offset. Symlinks are not followed. An empty
// needle list, or an empty needle, is an error: a scan that cannot find
// anything must not look like a clean result.
func ScanTree(root string, needles []string) ([]Hit, error) {
	if len(needles) == 0 {
		return nil, errors.New("zdr: no needles given")
	}
	for _, n := range needles {
		if n == "" {
			return nil, errors.New("zdr: empty needle")
		}
	}
	var hits []Hit
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("zdr: read %s: %w", path, err)
		}
		for _, n := range needles {
			nb := []byte(n)
			for off := 0; ; {
				i := bytes.Index(data[off:], nb)
				if i < 0 {
					break
				}
				hits = append(hits, Hit{Path: path, Offset: int64(off + i), Needle: n})
				off += i + 1
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Path != hits[j].Path {
			return hits[i].Path < hits[j].Path
		}
		if hits[i].Offset != hits[j].Offset {
			return hits[i].Offset < hits[j].Offset
		}
		return hits[i].Needle < hits[j].Needle
	})
	return hits, nil
}
