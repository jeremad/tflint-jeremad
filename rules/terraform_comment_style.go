package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

type TerraformCommentStyleRule struct {
	baseRule
}

func NewTerraformCommentStyleRule() *TerraformCommentStyleRule {
	return &TerraformCommentStyleRule{baseRule{name: "terraform_comment_style"}}
}

func (r *TerraformCommentStyleRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
}

func stripCommentPrefix(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "# ") {
		return trimmed[2:]
	}
	if strings.HasPrefix(trimmed, "#") {
		return trimmed[1:]
	}
	if strings.HasPrefix(trimmed, "// ") {
		return trimmed[3:]
	}
	if strings.HasPrefix(trimmed, "//") {
		return trimmed[2:]
	}
	return trimmed
}

func buildCommentFix(filename string, commentLines []string, byteStart, byteEnd int, startLine, endLine int) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		indent := ""
		first := commentLines[0]
		idx := strings.IndexFunc(first, func(r rune) bool { return r != ' ' && r != '\t' })
		if idx > 0 {
			indent = first[:idx]
		}

		var buf strings.Builder
		buf.WriteString(indent + "/*\n")
		for _, line := range commentLines {
			content := stripCommentPrefix(line)
			if content == "" {
				buf.WriteString(indent + " *\n")
			} else {
				buf.WriteString(indent + " * " + content + "\n")
			}
		}
		buf.WriteString(indent + " */")

		rng := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: startLine, Column: 1, Byte: byteStart},
			End:      hcl.Pos{Line: endLine, Column: 1, Byte: byteEnd},
		}
		return replaceTextOrConflict(f, rng, buf.String())
	}
}

func (r *TerraformCommentStyleRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	lines := strings.Split(string(file.Bytes), "\n")
	filename := body.SrcRange.Filename
	lineOffsets := computeLineOffsets(lines)

	emitRun := func(runStart, runEnd int) error {
		commentLines := lines[runStart:runEnd]
		byteStart := lineOffsets[runStart]
		byteEnd := lineOffsets[runEnd-1] + len(lines[runEnd-1])

		issueRange := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: runStart + 1, Column: 1},
			End:      hcl.Pos{Line: runEnd + 1, Column: 1},
		}
		fix := buildCommentFix(filename, commentLines, byteStart, byteEnd, runStart+1, runEnd+1)
		return runner.EmitIssueWithFix(
			r,
			"consecutive line comments should use block comment syntax (/* ... */)",
			issueRange,
			fix,
		)
	}

	// A run of consecutive line-comment lines becomes a candidate for
	// conversion to a block comment when it is at least 3 lines long, or
	// at least 2 lines long if any of them uses // (since // has a
	// dedicated single-line rule we want to nudge users away from).
	runStart := -1
	hasSlash := false
	threshold := func() int {
		if hasSlash {
			return 2
		}
		return 3
	}
	for i, line := range lines {
		if startsLineComment(strings.TrimSpace(line)) {
			if runStart == -1 {
				runStart = i
				hasSlash = false
			}
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				hasSlash = true
			}
			continue
		}
		if runStart != -1 && i-runStart >= threshold() {
			if err := emitRun(runStart, i); err != nil {
				return err
			}
		}
		runStart = -1
	}

	if runStart != -1 && len(lines)-runStart >= threshold() {
		if err := emitRun(runStart, len(lines)); err != nil {
			return err
		}
	}

	return nil
}
