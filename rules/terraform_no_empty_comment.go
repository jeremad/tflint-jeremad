package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

type TerraformNoEmptyCommentRule struct {
	baseRule
}

func NewTerraformNoEmptyCommentRule() *TerraformNoEmptyCommentRule {
	return &TerraformNoEmptyCommentRule{baseRule{name: "terraform_no_empty_comment"}}
}

func (r *TerraformNoEmptyCommentRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
}

func isEmptyComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "#" || trimmed == "//" || trimmed == "/**/" || trimmed == "/* */"
}

func (r *TerraformNoEmptyCommentRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	lines := strings.Split(string(file.Bytes), "\n")
	filename := body.SrcRange.Filename
	lineOffsets := computeLineOffsets(lines)

	// Coalesce runs of consecutive empty comments into a single fix so that
	// applying one fix cannot conflict with an adjacent one.
	for i := 0; i < len(lines); {
		if !isEmptyComment(lines[i]) {
			i++
			continue
		}
		runStart := i
		for i < len(lines) && isEmptyComment(lines[i]) {
			i++
		}
		runEnd := i // exclusive

		byteStart := lineOffsets[runStart]
		byteEnd := lineOffsets[runEnd-1] + len(lines[runEnd-1])
		if runEnd < len(lines) {
			byteEnd++ // consume the trailing newline
		}

		issueRange := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: runStart + 1, Column: 1},
			End:      hcl.Pos{Line: runEnd + 1, Column: 1},
		}

		// Loop-local copies are unnecessary under Go 1.22 (per-iteration
		// scope), but we shadow anyway so the closure is obviously safe
		// regardless of toolchain.
		filename, byteStart, byteEnd, runStart, runEnd := filename, byteStart, byteEnd, runStart, runEnd
		fix := func(f tflint.Fixer) error {
			rng := hcl.Range{
				Filename: filename,
				Start:    hcl.Pos{Line: runStart + 1, Column: 1, Byte: byteStart},
				End:      hcl.Pos{Line: runEnd + 1, Column: 1, Byte: byteEnd},
			}
			return replaceTextOrConflict(f, rng, "")
		}

		if err := runner.EmitIssueWithFix(
			r,
			"empty comments should be removed",
			issueRange,
			fix,
		); err != nil {
			return err
		}
	}

	return nil
}
