package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

func init() {
	Register("policy", "evaluate the policy for one tool request: eval", runPolicy)
}

func runPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		policyUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "eval":
		return policyEval(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		policyUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay policy: unknown action %q\n", args[0])
	policyUsage(stderr)
	return ExitError
}

func policyUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay policy eval --policy FILE --tool NAME --manifests DIR --agent A")
	fmt.Fprintln(w, "                    [--trust DIR | --assume-signed] [--lab L] [--depth N]")
	fmt.Fprintln(w, "                    [--ancestors a,b] [--spent USD]")
	fmt.Fprintln(w, "  prints the Decision as JSON; exit 0 allow, 1 deny, 2 error")
}

func policyEval(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy eval", stderr)
	policyPath := fs.String("policy", "", "policy YAML file")
	toolName := fs.String("tool", "", "tool name (manifest \"name\" field)")
	manifests := fs.String("manifests", "", "directory of *.json manifests")
	agent := fs.String("agent", "", "requesting agent id")
	lab := fs.String("lab", "", "lab id")
	depth := fs.Int("depth", 0, "delegation depth")
	ancestors := fs.String("ancestors", "", "comma-separated agent ids up the delegation chain")
	spent := fs.Float64("spent", 0, "USD already spent by the agent in this run")
	trust := fs.String("trust", "", "trust store directory; the manifest signature is verified against it")
	assume := fs.Bool("assume-signed", false, "skip signature verification and treat the manifest as signed (policy testing only)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *toolName == "" || *manifests == "" || *agent == "" {
		fmt.Fprintln(stderr, "assay policy eval: --policy, --tool, --manifests and --agent are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	var found *model.ToolManifest
	for i := range files {
		if files[i].M.Name == *toolName {
			found = &files[i].M
			break
		}
	}
	if found == nil {
		fmt.Fprintf(stderr, "assay policy eval: no manifest named %q in %s\n", *toolName, *manifests)
		return ExitError
	}

	req := model.PolicyRequest{
		Agent:    *agent,
		Tool:     *found,
		Lab:      *lab,
		Depth:    *depth,
		SpentUSD: *spent,
	}
	if *ancestors != "" {
		for _, a := range strings.Split(*ancestors, ",") {
			if a = strings.TrimSpace(a); a != "" {
				req.Ancestors = append(req.Ancestors, a)
			}
		}
	}
	switch {
	case *trust != "":
		ts, err := toolsig.LoadTrustStore(*trust)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
			return ExitError
		}
		_, reason, verr := ts.Verify(*found)
		req.SignatureOK = verr == nil
		req.SignatureReason = reason
	case *assume:
		req.SignatureOK = true
		req.SignatureReason = "ASSUMED_SIGNED"
	default:
		req.SignatureOK = false
		req.SignatureReason = policy.ReasonSignatureUnverified
	}

	d := eng.Evaluate(context.Background(), req)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	if d.Effect == model.EffectAllow {
		return ExitPass
	}
	return ExitBlocked
}
