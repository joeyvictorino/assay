package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/validate"
)

func init() {
	Register("checkers", "list available validation checkers", runCheckers)
}

func runCheckers(_ []string, stdout, _ io.Writer) int {
	infos := validate.CheckerInfos()
	if len(infos) == 0 {
		fmt.Fprintln(stdout, "No checkers registered.")
		return ExitPass
	}

	maxClass := len("CLASS")
	maxDesc := len("DESCRIPTION")
	for _, info := range infos {
		if n := len(string(info.Class)); n > maxClass {
			maxClass = n
		}
		if n := len(info.Description); n > maxDesc {
			maxDesc = n
		}
	}

	fmtStr := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", maxClass, maxDesc)
	fmt.Fprintf(stdout, fmtStr, "CLASS", "DESCRIPTION", "VALIDATES")
	fmt.Fprintf(stdout, "  %s  %s  %s\n",
		strings.Repeat("-", maxClass),
		strings.Repeat("-", maxDesc),
		strings.Repeat("-", 40))
	for _, info := range infos {
		fmt.Fprintf(stdout, fmtStr, string(info.Class), info.Description, info.Validates)
	}
	fmt.Fprintf(stdout, "\n%d checkers registered.\n", len(infos))
	return ExitPass
}
