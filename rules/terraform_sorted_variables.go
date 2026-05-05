package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

var varAttrOrder = map[string]int{
	"type":        0,
	"default":     1,
	"description": 2,
}

type TerraformSortedVariablesRule struct {
	tflint.DefaultRule
}

func NewTerraformSortedVariablesRule() *TerraformSortedVariablesRule {
	return &TerraformSortedVariablesRule{}
}

func (r *TerraformSortedVariablesRule) Name() string {
	return "terraform_sorted_variables"
}

func (r *TerraformSortedVariablesRule) Enabled() bool {
	return true
}

func (r *TerraformSortedVariablesRule) Severity() tflint.Severity {
	return tflint.WARNING
}

func (r *TerraformSortedVariablesRule) Link() string {
	return ""
}

func (r *TerraformSortedVariablesRule) Check(runner tflint.Runner) error {
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

func (r *TerraformSortedVariablesRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	for _, block := range body.Blocks {
		if block.Type != "variable" {
			continue
		}
		if err := r.checkVariable(runner, block, file.Bytes); err != nil {
			return err
		}
	}
	return nil
}

func buildVariableFix(items []bodyItem, bodyStart *hcl.Pos) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		return applyFix(f, func() error {
			if len(items) < 2 {
				return nil
			}

			rich := attachComments(items, f, bodyStart)

			sorted := make([]richItem, len(rich))
			copy(sorted, rich)
			sort.SliceStable(sorted, func(i, j int) bool {
				iOrder, iKnown := varAttrOrder[sorted[i].name]
				jOrder, jKnown := varAttrOrder[sorted[j].name]
				if iKnown && jKnown {
					return iOrder < jOrder
				}
				if iKnown {
					return true
				}
				if jKnown {
					return false
				}
				return false
			})

			indent := strings.Repeat(" ", items[0].fullRange.Start.Column-1)
			for i := range sorted {
				sorted[i].text = f.TextAt(sorted[i].fullRange).Bytes
			}

			var buf strings.Builder
			writeItems(&buf, sorted, indent, func(item, prev richItem) bool {
				_, prevKnown := varAttrOrder[prev.name]
				_, curKnown := varAttrOrder[item.name]
				return !(prevKnown && curKnown)
			})

			spanStart := items[0].fullRange.Start
			if len(rich[0].comments) > 0 && bodyStart != nil {
				spanStart = findCommentStart(string(f.TextAt(hcl.Range{
					Filename: items[0].fullRange.Filename,
					Start:    *bodyStart,
					End:      items[0].fullRange.Start,
				}).Bytes), *bodyStart)
			}

			spanRange := hcl.Range{
				Filename: items[0].fullRange.Filename,
				Start:    spanStart,
				End:      items[len(items)-1].fullRange.End,
			}
			return replaceTextOrConflict(f, spanRange, buf.String())
		})
	}
}

func gapHasBlankLine(src []byte, prev, item bodyItem) bool {
	if item.startLine <= prev.endLine+1 {
		return false
	}
	gap := string(src[prev.fullRange.End.Byte:item.fullRange.Start.Byte])
	lines := strings.Split(gap, "\n")
	// Skip first element (remainder of prev line) and last element (indent of next item).
	if len(lines) < 3 {
		return false
	}
	for _, line := range lines[1 : len(lines)-1] {
		if strings.TrimSpace(line) == "" {
			return true
		}
	}
	return false
}

func (r *TerraformSortedVariablesRule) checkVariable(runner tflint.Runner, block *hclsyntax.Block, src []byte) error {
	items := collectBodyItems(block.Body, src)
	bodyStart := block.OpenBraceRange.End
	fix := buildVariableFix(items, &bodyStart)

	for i, item := range items {
		if i == 0 {
			continue
		}
		prev := items[i-1]

		curOrder, curKnown := varAttrOrder[item.name]
		prevOrder, prevKnown := varAttrOrder[prev.name]

		if curKnown && prevKnown && curOrder < prevOrder {
			msg := fmt.Sprintf(
				"argument %q should come before %q in variable blocks (required order: type → default → description)",
				item.name, prev.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		_, itemIsVarAttr := varAttrOrder[item.name]
		_, prevIsVarAttr := varAttrOrder[prev.name]
		if itemIsVarAttr && prevIsVarAttr && gapHasBlankLine(src, prev, item) {
			msg := fmt.Sprintf(
				"unexpected blank line before %q: variable block attributes should not be separated by blank lines",
				item.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}
	}

	return nil
}
