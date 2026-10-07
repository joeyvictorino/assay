package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

// Key file names written by `assay tools keygen`.
const (
	PrivateKeyFile = "assay.key"
	PublicKeyFile  = "assay.pub"
)

func init() {
	Register("tools", "manage signed tool manifests: keygen | sign | verify | list", runTools)
}

func runTools(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		toolsUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "keygen":
		return toolsKeygen(args[1:], stdout, stderr)
	case "sign":
		return toolsSign(args[1:], stdout, stderr)
	case "verify":
		return toolsVerify(args[1:], stdout, stderr)
	case "list":
		return toolsList(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		toolsUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay tools: unknown action %q\n", args[0])
	toolsUsage(stderr)
	return ExitError
}

func toolsUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay tools keygen --out DIR                  write assay.key (0600) and assay.pub")
	fmt.Fprintln(w, "  assay tools sign --manifests DIR --key FILE   sign every *.json manifest in place")
	fmt.Fprintln(w, "  assay tools verify --manifests DIR --trust DIR  verify every manifest; exit 2 on any failure")
	fmt.Fprintln(w, "  assay tools list --manifests DIR              list manifests")
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func toolsKeygen(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tools keygen", stderr)
	out := fs.String("out", "", "directory to write assay.key and assay.pub into")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *out == "" {
		fmt.Fprintln(stderr, "assay tools keygen: --out is required")
		return ExitError
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		fmt.Fprintf(stderr, "assay tools keygen: %v\n", err)
		return ExitError
	}
	pub, priv := toolsig.GenerateKey()
	privPath := filepath.Join(*out, PrivateKeyFile)
	pubPath := filepath.Join(*out, PublicKeyFile)
	// O_EXCL: never overwrite an existing key.
	f, err := os.OpenFile(privPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools keygen: %v\n", err)
		return ExitError
	}
	_, werr := io.WriteString(f, toolsig.EncodePrivateKey(priv)+"\n")
	cerr := f.Close()
	if werr != nil || cerr != nil {
		fmt.Fprintf(stderr, "assay tools keygen: write %s: %v %v\n", privPath, werr, cerr)
		return ExitError
	}
	if err := os.WriteFile(pubPath, []byte(toolsig.EncodePublicKey(pub)+"\n"), 0o644); err != nil {
		fmt.Fprintf(stderr, "assay tools keygen: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "key id %s\nprivate %s\npublic  %s\n", toolsig.KeyID(pub), privPath, pubPath)
	return ExitPass
}

// manifestFile pairs a manifest with the file it was read from.
type manifestFile struct {
	Path string
	M    model.ToolManifest
}

// loadManifestDir reads every *.json manifest in dir, strictly.
func loadManifestDir(dir string) ([]manifestFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []manifestFile
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var m model.ToolManifest
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if strings.TrimSpace(m.Name) == "" {
			return nil, fmt.Errorf("%s: manifest has no name", p)
		}
		out = append(out, manifestFile{Path: p, M: m})
	}
	if len(out) == 0 {
		return nil, errors.New("no *.json manifests in " + dir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func toolsSign(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tools sign", stderr)
	dir := fs.String("manifests", "", "directory of *.json manifests")
	keyPath := fs.String("key", "", "private key file (base64 seed)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *dir == "" || *keyPath == "" {
		fmt.Fprintln(stderr, "assay tools sign: --manifests and --key are required")
		return ExitError
	}
	raw, err := os.ReadFile(*keyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools sign: %v\n", err)
		return ExitError
	}
	priv, err := toolsig.ParsePrivateKey(string(raw))
	if err != nil {
		fmt.Fprintf(stderr, "assay tools sign: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools sign: %v\n", err)
		return ExitError
	}
	now := time.Now()
	var keyID string
	for _, mf := range files {
		signed, err := toolsig.Sign(mf.M, priv, now)
		if err != nil {
			fmt.Fprintf(stderr, "assay tools sign: %s: %v\n", mf.Path, err)
			return ExitError
		}
		b, err := json.MarshalIndent(signed, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "assay tools sign: %s: %v\n", mf.Path, err)
			return ExitError
		}
		if err := os.WriteFile(mf.Path, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintf(stderr, "assay tools sign: %v\n", err)
			return ExitError
		}
		keyID = signed.Signer
		fmt.Fprintf(stdout, "signed %-24s %s\n", signed.Name, filepath.Base(mf.Path))
	}
	fmt.Fprintf(stdout, "key id %s (%d manifests)\n", keyID, len(files))
	return ExitPass
}

func toolsVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tools verify", stderr)
	dir := fs.String("manifests", "", "directory of *.json manifests")
	trust := fs.String("trust", "", "trust store directory of *.pub files")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *dir == "" || *trust == "" {
		fmt.Fprintln(stderr, "assay tools verify: --manifests and --trust are required")
		return ExitError
	}
	ts, err := toolsig.LoadTrustStore(*trust)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools verify: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools verify: %v\n", err)
		return ExitError
	}
	failed := 0
	for _, mf := range files {
		keyID, reason, err := ts.Verify(mf.M)
		if err != nil {
			failed++
		}
		label := ts.Label(keyID)
		if label != "" {
			label = " (" + label + ")"
		}
		fmt.Fprintf(stdout, "%-24s %-18s %s%s\n", mf.M.Name, reason, keyID, label)
	}
	if failed > 0 {
		fmt.Fprintf(stderr, "assay tools verify: %d of %d manifests failed\n", failed, len(files))
		return ExitError
	}
	fmt.Fprintf(stdout, "%d manifests verified\n", len(files))
	return ExitPass
}

func toolsList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tools list", stderr)
	dir := fs.String("manifests", "", "directory of *.json manifests")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "assay tools list: --manifests is required")
		return ExitError
	}
	files, err := loadManifestDir(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "assay tools list: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "%-24s %-10s %-9s %-6s %-16s %s\n", "NAME", "VERSION", "TIER", "EFFECT", "SIGNER", "CAPABILITIES")
	for _, mf := range files {
		m := mf.M
		signer := m.Signer
		if m.Signature == "" {
			signer = "unsigned"
		}
		effect := "none"
		if m.SideEffects {
			effect = "yes"
		}
		fmt.Fprintf(stdout, "%-24s %-10s %-9s %-6s %-16s %s\n", m.Name, m.Version, m.RiskTier, effect, signer, strings.Join(m.Capabilities, ","))
	}
	return ExitPass
}
