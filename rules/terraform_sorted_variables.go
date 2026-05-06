package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

var varFixedOrder = map[string]int{
	"type":        0,
	"default":     1,
	"description": 2,
}

const (
	varCatFixed      = 0
	varCatOther      = 1
	varCatValidation = 2
)

func varCategory(name string) int {
	if _, ok := varFixedOrder[name]; ok {
		return varCatFixed
	}
	if name == "validation" {
		return varCatValidation
	}
	return varCatOther
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

func buildVariableFix(items []bodyItem, bodyStart hcl.Pos) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		return applyFix(f, func() error {
			if len(items) < 2 {
				return nil
			}

			rich := attachComments(items, f, bodyStart)

			sorted := make([]richItem, len(rich))
			copy(sorted, rich)
			sort.SliceStable(sorted, func(i, j int) bool {
				ci, cj := varCategory(sorted[i].name), varCategory(sorted[j].name)
				if ci != cj {
					return ci < cj
				}
				if ci == varCatFixed {
					return varFixedOrder[sorted[i].name] < varFixedOrder[sorted[j].name]
				}
				if ci == varCatOther {
					return sorted[i].name < sorted[j].name
				}
				return false
			})

			indent := strings.Repeat(" ", items[0].fullRange.Start.Column-1)
			for i := range sorted {
				sorted[i].text = f.TextAt(sorted[i].fullRange).Bytes
			}

			var buf strings.Builder
			writeItems(&buf, sorted, indent, func(item, prev richItem) bool {
				return varCategory(item.name) == varCatValidation
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
		prevCat, curCat := varCategory(prev.name), varCategory(item.name)

		// Category ordering: fixed < other < validation.
		if curCat < prevCat {
			msg := fmt.Sprintf(
				"argument %q should come before %q in variable blocks",
				item.name, prev.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
			continue
		}

		// Within fixed-order group: type → default → description.
		if curCat == varCatFixed && prevCat == varCatFixed {
			if varFixedOrder[item.name] < varFixedOrder[prev.name] {
				msg := fmt.Sprintf(
					"argument %q should come before %q in variable blocks (required order: type → default → description)",
					item.name, prev.name,
				)
				if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
					return err
				}
			}
		}

		// Within other group: alphabetical.
		if curCat == varCatOther && prevCat == varCatOther && item.name < prev.name {
			msg := fmt.Sprintf(
				"argument %q is not sorted: it should come before %q",
				item.name, prev.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		// No blank lines except before validation.
		if curCat != varCatValidation && gapHasBlankLine(src, prev, item) {
			msg := fmt.Sprintf(
				"unexpected blank line before %q: variable block attributes should not be separated by blank lines",
				item.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		// Blank line required before validation.
		if curCat == varCatValidation && !gapHasBlankLine(src, prev, item) && item.startLine <= prev.endLine+1 {
			msg := "missing blank line before \"validation\" block"
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}
	}

	return nil
}
