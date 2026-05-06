package rules

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

type TerraformBlockCommentFormatRule struct {
	baseRule
}

func NewTerraformBlockCommentFormatRule() *TerraformBlockCommentFormatRule {
	return &TerraformBlockCommentFormatRule{baseRule{name: "terraform_block_comment_format"}}
}

func (r *TerraformBlockCommentFormatRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
}

type blockComment struct {
	startLine int
	endLine   int
	lines     []string
	indent    string
}

func findBlockComments(lines []string) []blockComment {
	var blocks []blockComment
	inBlock := false
	var current blockComment

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if idx := strings.Index(trimmed, "/*"); idx == 0 {
				indent := ""
				rawIdx := strings.Index(line, "/*")
				if rawIdx > 0 {
					indent = line[:rawIdx]
				}
				current = blockComment{
					startLine: i,
					indent:    indent,
					lines:     []string{line},
				}
				if strings.Contains(trimmed, "*/") {
					current.endLine = i
					blocks = append(blocks, current)
				} else {
					inBlock = true
				}
			}
		} else {
			current.lines = append(current.lines, line)
			if strings.Contains(trimmed, "*/") {
				current.endLine = i
				inBlock = false
				blocks = append(blocks, current)
			}
		}
	}
	return blocks
}

// stripBlockCommentContent extracts the human-readable text from one line of
// a block comment, regardless of where in the comment the line appears.
//
// Cases are checked in priority order, and order matters:
//  1. /** ...    — javadoc-style opener (must be checked before /*)
//  2. /* ...     — regular opener
//  3. * ...      — body line with leading star and space
//  4. *  /  ""   — body line that is bare star or empty
//  5. fallback   — strip any leading lone star and return what remains
func stripBlockCommentContent(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSuffix(trimmed, "*/")
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasPrefix(trimmed, "/**") {
		return strings.TrimSpace(trimmed[3:])
	}
	if strings.HasPrefix(trimmed, "/*") {
		return strings.TrimSpace(trimmed[2:])
	}
	if strings.HasPrefix(trimmed, "* ") {
		return trimmed[2:]
	}
	if trimmed == "*" || trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "*")
}

func validateBlockComment(bc blockComment) string {
	if bc.startLine == bc.endLine {
		return ""
	}

	openLine := strings.TrimSpace(bc.lines[0])
	if strings.HasPrefix(openLine, "/**") {
		return "block comment should start with /* not /**"
	}
	if openLine != "/*" {
		return "block comment opening /* should be on its own line"
	}

	starCol := len(bc.indent) + 1

	for _, line := range bc.lines[1 : len(bc.lines)-1] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		starIdx := strings.Index(line, "*")
		if starIdx < 0 || starIdx != starCol {
			return "block comment * characters are not aligned"
		}
		rest := line[starIdx+1:]
		if rest != "" && !strings.HasPrefix(rest, " ") {
			return "block comment lines should use * followed by a space"
		}
	}

	closeLine := bc.lines[len(bc.lines)-1]
	closeTrimmed := strings.TrimSpace(closeLine)
	if closeTrimmed != "*/" {
		return "block comment closing */ should be on its own line"
	}
	closeStarIdx := strings.Index(closeLine, "*")
	if closeStarIdx != starCol {
		return "block comment * characters are not aligned"
	}

	if len(bc.lines) >= 3 {
		prevLine := strings.TrimSpace(bc.lines[len(bc.lines)-2])
		if prevLine == "*" {
			return "block comment should not have trailing empty lines before */"
		}
	}

	return ""
}

func buildBlockCommentFix(bc blockComment, filename string, lineOffsets []int) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		var contents []string

		if first := stripBlockCommentContent(bc.lines[0]); first != "" {
			contents = append(contents, first)
		}
		if len(bc.lines) > 2 {
			for _, line := range bc.lines[1 : len(bc.lines)-1] {
				contents = append(contents, stripBlockCommentContent(line))
			}
		}
		if len(bc.lines) > 1 {
			if last := stripBlockCommentContent(bc.lines[len(bc.lines)-1]); last != "" {
				contents = append(contents, last)
			}
		}

		for len(contents) > 0 && contents[len(contents)-1] == "" {
			contents = contents[:len(contents)-1]
		}

		var buf strings.Builder
		buf.WriteString(bc.indent + "/*\n")
		for _, content := range contents {
			if content == "" {
				buf.WriteString(bc.indent + " *\n")
			} else {
				buf.WriteString(bc.indent + " * " + content + "\n")
			}
		}
		buf.WriteString(bc.indent + " */")

		byteStart := lineOffsets[bc.startLine]
		byteEnd := lineOffsets[bc.endLine] + len(bc.lines[len(bc.lines)-1])

		rng := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: bc.startLine + 1, Column: 1, Byte: byteStart},
			End:      hcl.Pos{Line: bc.endLine + 1, Column: len(bc.lines[len(bc.lines)-1]) + 1, Byte: byteEnd},
		}
		return replaceTextOrConflict(f, rng, buf.String())
	}
}

func (r *TerraformBlockCommentFormatRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	lines := strings.Split(string(file.Bytes), "\n")
	filename := body.SrcRange.Filename
	lineOffsets := computeLineOffsets(lines)

	blocks := findBlockComments(lines)
	for _, bc := range blocks {
		msg := validateBlockComment(bc)
		if msg == "" {
			continue
		}

		issueRange := hcl.Range{
			Filename: filename,
			Start:    hcl.Pos{Line: bc.startLine + 1, Column: 1},
			End:      hcl.Pos{Line: bc.endLine + 2, Column: 1},
		}
		fix := buildBlockCommentFix(bc, filename, lineOffsets)
		if err := runner.EmitIssueWithFix(r, msg, issueRange, fix); err != nil {
			return err
		}
	}

	return nil
}
