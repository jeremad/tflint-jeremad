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
	baseRule
}

func NewTerraformSortedVariablesRule() *TerraformSortedVariablesRule {
	return &TerraformSortedVariablesRule{baseRule{name: "terraform_sorted_variables"}}
}

func (r *TerraformSortedVariablesRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
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

// buildVariableFix returns a Fixer callback that rewrites a variable block's
// attributes in canonical order (type → default → description, then any
// other attributes in source order). Like buildBlockFix, the output relies
// on hclwrite.Format upstream to re-align whitespace.
func buildVariableFix(items []bodyItem, bodyStart hcl.Pos) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		return applyFix(f, func() error {
			if len(items) < 2 {
				return nil
			}

			rich := attachComments(items, f, bodyStart)

			sorted := make([]richItem, len(rich))
			copy(sorted, rich)
			// Stable sort: among attributes not in varAttrOrder we
			// preserve source order.
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
			if len(rich[0].comments) > 0 {
				spanStart = findCommentStart(string(f.TextAt(hcl.Range{
					Filename: items[0].fullRange.Filename,
					Start:    bodyStart,
					End:      items[0].fullRange.Start,
				}).Bytes), bodyStart)
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
	fix := buildVariableFix(items, block.OpenBraceRange.End)

	for i := 1; i < len(items); i++ {
		prev, item := items[i-1], items[i]

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

		if curKnown && prevKnown && gapHasBlankLine(src, prev, item) {
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
