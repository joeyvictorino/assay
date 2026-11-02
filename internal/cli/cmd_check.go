package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/validate"
)

func init() {
	Register("check", "run a single checker against one manifest", runCheck)
}

type checkResult struct {
	Checker string                 `json:"checker"`
	Class   string                 `json:"class"`
	State   string                 `json:"state"`
	Note    string                 `json:"note,omitempty"`
	Result  *model.ValidationResult `json:"result,omitempty"`
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay check", stderr)
	checkerName := fs.String("checker", "", "checker class name (required, e.g. sqli, xss-reflected)")
	manifestPath := fs.String("manifest", "", "path to a single manifest JSON file (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *checkerName == "" || *manifestPath == "" {
		fmt.Fprintln(stderr, "assay check: --checker and --manifest are required")
		return ExitError
	}

	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay check: %v\n", err)
		return ExitError
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m model.ToolManifest
	if err := dec.Decode(&m); err != nil {
		fmt.Fprintf(stderr, "assay check: %s: %v\n", *manifestPath, err)
		return ExitError
	}

	class := model.Class(normalizeCheckerName(*checkerName))
	checker := validate.Lookup(class)
	if checker == nil {
		fmt.Fprintf(stderr, "assay check: unknown checker %q\n", *checkerName)
		fmt.Fprintln(stderr, "available checkers:")
		for _, info := range validate.CheckerInfos() {
			fmt.Fprintf(stderr, "  %s\n", info.Class)
		}
		return ExitError
	}

	finding := model.Finding{
		Class:   class,
		Summary: "check against " + m.Name,
	}

	res, checkErr := checker.Check(context.Background(), finding, validate.Env{})

	cr := checkResult{
		Checker: string(class),
		Class:   string(class),
		State:   string(res.State),
		Note:    res.Note,
	}
	if checkErr != nil {
		cr.State = "error"
		cr.Note = checkErr.Error()
	}
	if res.State != "" {
		cr.Result = &res
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cr); err != nil {
			fmt.Fprintf(stderr, "assay check: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprintf(stdout, "Checker:  %s\n", cr.Checker)
		fmt.Fprintf(stdout, "Manifest: %s (%s)\n", m.Name, *manifestPath)
		fmt.Fprintf(stdout, "State:    %s\n", cr.State)
		if cr.Note != "" {
			fmt.Fprintf(stdout, "Note:     %s\n", cr.Note)
		}
	default:
		fmt.Fprintf(stderr, "assay check: unknown format %q\n", *format)
		return ExitError
	}

	switch {
	case checkErr != nil:
		return ExitError
	case res.State == model.StateValidated:
		return ExitBlocked
	default:
		return ExitPass
	}
}

func normalizeCheckerName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "-", "_")
	return name
}
