package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

type TerraformSingleLineCommentStyleRule struct {
	tflint.DefaultRule
}

func NewTerraformSingleLineCommentStyleRule() *TerraformSingleLineCommentStyleRule {
	return &TerraformSingleLineCommentStyleRule{}
}

func (r *TerraformSingleLineCommentStyleRule) Name() string {
	return "terraform_single_line_comment_style"
}

func (r *TerraformSingleLineCommentStyleRule) Enabled() bool {
	return true
}

func (r *TerraformSingleLineCommentStyleRule) Severity() tflint.Severity {
	return tflint.WARNING
}

func (r *TerraformSingleLineCommentStyleRule) Link() string {
	return ""
}

func (r *TerraformSingleLineCommentStyleRule) Check(runner tflint.Runner) error {
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

func (r *TerraformSingleLineCommentStyleRule) checkFile(runner tflint.Runner, file *hcl.File) error {
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
		trimmed := strings.TrimSpace(line)

		var content, indent, msg string
		if strings.HasPrefix(trimmed, "/*") && strings.HasSuffix(trimmed, "*/") &&
			strings.Count(line, "/*") == 1 && strings.Count(line, "*/") == 1 {
			content = strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			idx := strings.Index(line, "/*")
			if idx > 0 {
				indent = line[:idx]
			}
			msg = "single-line block comments should use # syntax"
		} else if strings.HasPrefix(trimmed, "//") {
			if strings.HasPrefix(trimmed, "// ") {
				content = trimmed[3:]
			} else {
				content = trimmed[2:]
			}
			idx := strings.Index(line, "//")
			if idx > 0 {
				indent = line[:idx]
			}
			msg = "single-line // comments should use # syntax"
		} else {
			continue
		}

		if content == "" {
			continue
		}
		replacement := indent + "# " + content

		byteStart := lineOffsets[i]
		byteEnd := byteStart + len(line)

		issueRange := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: i + 1, Column: 1},
			End:      hcl.Pos{Line: i + 2, Column: 1},
		}

		fix := func(f tflint.Fixer) error {
			rng := hcl.Range{
				Filename: filename,
				Start:    hcl.Pos{Line: i + 1, Column: 1, Byte: byteStart},
				End:      hcl.Pos{Line: i + 1, Column: len(line) + 1, Byte: byteEnd},
			}
			return replaceTextOrConflict(f, rng, replacement)
		}

		if err := runner.EmitIssueWithFix(r, msg, issueRange, fix); err != nil {
			return err
		}
	}

	return nil
}
