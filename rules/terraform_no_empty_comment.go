package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

type TerraformNoEmptyCommentRule struct {
	tflint.DefaultRule
}

func NewTerraformNoEmptyCommentRule() *TerraformNoEmptyCommentRule {
	return &TerraformNoEmptyCommentRule{}
}

func (r *TerraformNoEmptyCommentRule) Name() string {
	return "terraform_no_empty_comment"
}

func (r *TerraformNoEmptyCommentRule) Enabled() bool {
	return true
}

func (r *TerraformNoEmptyCommentRule) Severity() tflint.Severity {
	return tflint.WARNING
}

func (r *TerraformNoEmptyCommentRule) Link() string {
	return ""
}

func (r *TerraformNoEmptyCommentRule) Check(runner tflint.Runner) error {
	files, err := runner.GetFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := r.checkFile(runner, file); err != nil {
			return err
		}
	}
	return nil
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

	src := file.Bytes
	lines := strings.Split(string(src), "\n")
	filename := body.SrcRange.Filename

	lineOffsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1
	}

	for i, line := range lines {
		if !isEmptyComment(line) {
			continue
		}

		byteStart := lineOffsets[i]
		byteEnd := byteStart + len(line)
		if i+1 < len(lines) {
			byteEnd++
		}

		issueRange := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: i + 1, Column: 1},
			End:      hcl.Pos{Line: i + 2, Column: 1},
		}

		fix := func(f tflint.Fixer) error {
			rng := hcl.Range{
				Filename: filename,
				Start:    hcl.Pos{Line: i + 1, Column: 1, Byte: byteStart},
				End:      hcl.Pos{Line: i + 2, Column: 1, Byte: byteEnd},
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
