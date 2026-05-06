package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

// baseRule provides shared boilerplate for every rule in this package:
// Name (set at construction), Enabled, Severity, and Link.
type baseRule struct {
	tflint.DefaultRule
	name string
}

func (r *baseRule) Name() string              { return r.name }
func (r *baseRule) Enabled() bool             { return true }
func (r *baseRule) Severity() tflint.Severity { return tflint.WARNING }
func (r *baseRule) Link() string              { return "" }

// forEachFile invokes fn for every file the runner exposes, returning the
// first error encountered.
func forEachFile(runner tflint.Runner, fn func(*hcl.File) error) error {
	files, err := runner.GetFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := fn(file); err != nil {
			return err
		}
	}
	return nil
}

// computeLineOffsets returns the byte offset of the start of each line, given
// the lines produced by strings.Split(src, "\n").
func computeLineOffsets(lines []string) []int {
	offsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len(line) + 1
	}
	return offsets
}

// startsLineComment reports whether s (typically already trimmed of leading
// whitespace) begins with a `#` or `//` line comment.
func startsLineComment(s string) bool {
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//")
}

// startsAnyComment is startsLineComment plus block-comment openers (`/*`).
func startsAnyComment(s string) bool {
	return startsLineComment(s) || strings.HasPrefix(s, "/*")
}
