package rules

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

// Argument categories — items must appear in non-decreasing order within a block.
//
//  0  provider                 the `provider` attribute
//  1  instantiation meta-args  `count`, `for_each`
//  2  source                   the `source` attribute (modules)
//  3  primitives               bool/number/string scalars, references, function calls
//  4  complex                  list `[…]` or map `{…}` values
//  5  nested blocks            HCL sub-blocks (lifecycle/depends_on excluded)
//  6  lifecycle meta-args      `lifecycle` block, `depends_on`, etc.
const (
	catProvider      = 0
	catInstantiation = 1
	catSource        = 2
	catPrimitive     = 3
	catComplex       = 4
	catBlock         = 5
	catLifecycle     = 6
)

var catLabel = map[int]string{
	catProvider:      "provider",
	catInstantiation: "instantiation meta-argument (count/for_each)",
	catSource:        "source",
	catPrimitive:     "primitive variable",
	catComplex:       "complex variable (list/map)",
	catBlock:         "nested block",
	catLifecycle:     "lifecycle meta-argument",
}

const orderingHint = "required order: provider → count/for_each → source → " +
	"primitive variables → complex variables → nested blocks → lifecycle meta-arguments"

// topMetaAttrs maps attribute names to their fixed top-of-block category.
var topMetaAttrs = map[string]int{
	"provider": catProvider,
	"count":    catInstantiation,
	"for_each": catInstantiation,
	"source":   catSource,
}

// endMetaNames are attribute or block names that always live at the bottom
// of a block body, in the lifecycle meta-arguments section.
var endMetaNames = map[string]bool{
	"depends_on": true,
	"lifecycle":  true,
}

func categorizeAttr(name string, expr hclsyntax.Expression) int {
	if cat, ok := topMetaAttrs[name]; ok {
		return cat
	}
	if endMetaNames[name] {
		return catLifecycle
	}
	switch expr.(type) {
	case *hclsyntax.ObjectConsExpr, *hclsyntax.TupleConsExpr:
		return catComplex
	default:
		return catPrimitive
	}
}

func categorizeBlock(blockType string) int {
	if endMetaNames[blockType] {
		return catLifecycle
	}
	return catBlock
}

// bodyItem represents a single attribute or nested block in a body.
type bodyItem struct {
	name      string
	category  int
	startLine int
	endLine   int
	nameRange hcl.Range
}

func collectBodyItems(body *hclsyntax.Body) []bodyItem {
	var items []bodyItem

	for name, attr := range body.Attributes {
		r := attr.Range()
		items = append(items, bodyItem{
			name:      name,
			category:  categorizeAttr(name, attr.Expr),
			startLine: r.Start.Line,
			endLine:   r.End.Line,
			nameRange: attr.NameRange,
		})
	}

	for _, block := range body.Blocks {
		r := block.Range()
		items = append(items, bodyItem{
			name:      block.Type,
			category:  categorizeBlock(block.Type),
			startLine: r.Start.Line,
			endLine:   r.End.Line,
			nameRange: block.TypeRange,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].startLine < items[j].startLine
	})
	return items
}

// needsBlankLineBefore returns true if a blank line must appear before item
// given the preceding item prev.
//
// Rules:
//   - Any forward category transition requires a blank line.
//   - Consecutive different-type nested blocks require a blank line.
//   - Consecutive lifecycle meta-arguments each require their own blank line.
func needsBlankLineBefore(item, prev bodyItem) bool {
	if item.category > prev.category {
		return true
	}
	if item.category == prev.category {
		switch item.category {
		case catBlock:
			return item.name != prev.name
		case catLifecycle:
			return true
		}
	}
	return false
}

// needsAlphaCheck returns true if alphabetical order must be enforced between
// prev and item. Consecutive same-type nested blocks are exempt (they form a
// logical group and their internal order is intentional).
func needsAlphaCheck(item, prev bodyItem) bool {
	if item.category != prev.category {
		return false
	}
	if item.category == catBlock && item.name == prev.name {
		return false
	}
	return true
}

// TerraformSortedArgumentsRule enforces the canonical argument ordering
// described in the team's Terraform style guide.
type TerraformSortedArgumentsRule struct {
	tflint.DefaultRule
}

func NewTerraformSortedArgumentsRule() *TerraformSortedArgumentsRule {
	return &TerraformSortedArgumentsRule{}
}

func (r *TerraformSortedArgumentsRule) Name() string {
	return "terraform_sorted_arguments"
}

func (r *TerraformSortedArgumentsRule) Enabled() bool {
	return true
}

func (r *TerraformSortedArgumentsRule) Severity() tflint.Severity {
	return tflint.WARNING
}

func (r *TerraformSortedArgumentsRule) Link() string {
	return ""
}

func (r *TerraformSortedArgumentsRule) Check(runner tflint.Runner) error {
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

func (r *TerraformSortedArgumentsRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	for _, block := range body.Blocks {
		if err := r.checkBlock(runner, block); err != nil {
			return err
		}
	}
	return nil
}

func (r *TerraformSortedArgumentsRule) checkBlock(runner tflint.Runner, block *hclsyntax.Block) error {
	items := collectBodyItems(block.Body)

	for i, item := range items {
		if i == 0 {
			continue
		}
		prev := items[i-1]

		// Rule 1 — category ordering.
		if item.category < prev.category {
			msg := fmt.Sprintf(
				"argument %q (%s) should come before %q (%s): %s",
				item.name, catLabel[item.category],
				prev.name, catLabel[prev.category],
				orderingHint,
			)
			if err := runner.EmitIssue(r, msg, item.nameRange); err != nil {
				return err
			}
		}

		// Rule 2 — alphabetical order within the same category.
		if needsAlphaCheck(item, prev) && item.name < prev.name {
			msg := fmt.Sprintf(
				"argument %q is not sorted: it should come before %q",
				item.name, prev.name,
			)
			if err := runner.EmitIssue(r, msg, item.nameRange); err != nil {
				return err
			}
		}

		// Rule 3 — blank line requirement.
		if needsBlankLineBefore(item, prev) && item.startLine <= prev.endLine+1 {
			msg := fmt.Sprintf(
				"missing blank line before %q (%s)",
				item.name, catLabel[item.category],
			)
			if err := runner.EmitIssue(r, msg, item.nameRange); err != nil {
				return err
			}
		}
	}

	// Recurse into nested blocks.
	for _, nested := range block.Body.Blocks {
		if err := r.checkBlock(runner, nested); err != nil {
			return err
		}
	}

	return nil
}
