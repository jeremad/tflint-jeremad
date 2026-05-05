package rules

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

// Argument categories — items must appear in non-decreasing order within a block.
//
//	0  provider                 the `provider` attribute
//	1  instantiation meta-args  `count`, `for_each`
//	2  source                   the `source` attribute (modules)
//	3  primitives               bool/number/string scalars, references, function calls
//	4  complex                  list `[…]` or map `{…}` values
//	5  nested blocks            HCL sub-blocks (lifecycle/depends_on excluded)
//	6  lifecycle meta-args      `lifecycle` block, `depends_on`, etc.
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

var complexFuncs = map[string]bool{
	"compact":      true,
	"concat":       true,
	"jsonencode":   true,
	"merge":        true,
	"templatefile": true,
	"toset":        true,
}

func categorizeAttr(name string, expr hclsyntax.Expression) int {
	if cat, ok := topMetaAttrs[name]; ok {
		return cat
	}
	if endMetaNames[name] {
		return catLifecycle
	}
	switch e := expr.(type) {
	case *hclsyntax.ObjectConsExpr, *hclsyntax.TupleConsExpr, *hclsyntax.ForExpr:
		return catComplex
	case *hclsyntax.FunctionCallExpr:
		if complexFuncs[e.Name] {
			return catComplex
		}
		return catPrimitive
	default:
		r := expr.Range()
		if r.End.Line > r.Start.Line {
			return catComplex
		}
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
	fullRange hcl.Range
}

func extendToLineEnd(src []byte, pos hcl.Pos) hcl.Pos {
	i := pos.Byte
	for i < len(src) && src[i] != '\n' {
		i++
	}
	end := strings.TrimRight(string(src[pos.Byte:i]), " \t")
	pos.Byte += len(end)
	pos.Column += len(end)
	return pos
}

func collectBodyItems(body *hclsyntax.Body, src []byte) []bodyItem {
	items := make([]bodyItem, 0, len(body.Attributes)+len(body.Blocks))

	for name, attr := range body.Attributes {
		r := attr.Range()
		exprEnd := attr.Expr.Range().End
		if src != nil {
			exprEnd = extendToLineEnd(src, exprEnd)
		}
		items = append(items, bodyItem{
			name:      name,
			category:  categorizeAttr(name, attr.Expr),
			startLine: r.Start.Line,
			endLine:   r.End.Line,
			nameRange: attr.NameRange,
			fullRange: hcl.Range{
				Filename: attr.NameRange.Filename,
				Start:    attr.NameRange.Start,
				End:      exprEnd,
			},
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
			fullRange: hcl.Range{
				Filename: block.TypeRange.Filename,
				Start:    block.TypeRange.Start,
				End:      block.CloseBraceRange.End,
			},
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
//   - Consecutive complex variables (lists/maps) each require a blank line.
//   - Consecutive different-type nested blocks require a blank line.
//   - Consecutive lifecycle meta-arguments each require their own blank line.
func needsBlankLineBefore(item, prev bodyItem) bool {
	if item.category > prev.category {
		if prev.category <= catSource && item.category <= catSource {
			return false
		}
		if item.category == catBlock && item.name == "content" {
			return false
		}
		return true
	}
	if item.category == prev.category {
		switch item.category {
		case catComplex:
			return true
		case catBlock:
			if item.name == "dynamic" {
				return true
			}
			return item.name != prev.name
		case catLifecycle:
			return true
		}
	}
	return false
}

var errFixConflict = errors.New("fix conflict")

func replaceTextOrConflict(f tflint.Fixer, rng hcl.Range, text string) error {
	if err := f.ReplaceText(rng, text); err != nil {
		return errors.Join(errFixConflict, err)
	}
	return nil
}

func applyFix(f tflint.Fixer, fn func() error) error {
	if err := fn(); err != nil {
		if errors.Is(err, errFixConflict) {
			return tflint.ErrFixNotSupported
		}
		return err
	}
	return nil
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

func blockHasViolations(items []bodyItem) bool {
	for i, item := range items {
		if i == 0 {
			continue
		}
		prev := items[i-1]
		if item.category < prev.category {
			return true
		}
		if needsAlphaCheck(item, prev) && item.name < prev.name {
			return true
		}
		if needsBlankLineBefore(item, prev) && item.startLine <= prev.endLine+1 {
			return true
		}
		if prev.category <= catSource && item.category <= catSource && item.startLine > prev.endLine+1 {
			return true
		}
	}
	return false
}

func (r *TerraformSortedArgumentsRule) checkFile(runner tflint.Runner, file *hcl.File) error {
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	for _, block := range body.Blocks {
		if block.Type == "variable" {
			continue
		}
		if err := r.checkBlock(runner, block, file.Bytes); err != nil {
			return err
		}
	}
	return nil
}

// extractCommentLines returns comment lines found in the gap text between two
// items. The first line of the gap (remainder of the previous item's line) is
// skipped so that inline comments stay with the preceding item.
func isCommentLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*")
}

func extractCommentLines(gap string) []string {
	lines := strings.Split(gap, "\n")
	var comments []string
	inBlock := false
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			comments = append(comments, trimmed)
			if strings.Contains(trimmed, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			inBlock = true
			comments = append(comments, trimmed)
			if strings.Contains(trimmed, "*/") {
				inBlock = false
			}
		} else if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			comments = append(comments, trimmed)
		}
	}
	return comments
}

// findCommentStart returns the position of the first comment line in gap text.
// pos is the starting position of the gap in the file.
func findCommentStart(gap string, pos hcl.Pos) hcl.Pos {
	lines := strings.Split(gap, "\n")
	cur := pos
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCommentLine(trimmed) {
			return cur
		}
		cur.Line++
		cur.Byte += len(line) + 1
		cur.Column = 1
	}
	return pos
}

type richItem struct {
	bodyItem
	comments []string
	text     []byte
}

func attachComments(items []bodyItem, f tflint.Fixer, bodyStart *hcl.Pos) []richItem {
	rich := make([]richItem, len(items))
	for i, item := range items {
		var comments []string
		var gapStart hcl.Pos
		if i > 0 {
			gapStart = items[i-1].fullRange.End
		} else if bodyStart != nil {
			gapStart = *bodyStart
		}
		if gapStart.Byte > 0 || i > 0 {
			gapRange := hcl.Range{
				Filename: item.fullRange.Filename,
				Start:    gapStart,
				End:      item.fullRange.Start,
			}
			gap := string(f.TextAt(gapRange).Bytes)
			if i == 0 {
				comments = extractCommentLines("\n" + gap)
			} else {
				comments = extractCommentLines(gap)
			}
		}
		rich[i] = richItem{bodyItem: item, comments: comments}
	}
	return rich
}

func writeItems(buf *strings.Builder, sorted []richItem, indent string, separator func(item, prev richItem) bool) {
	for i, item := range sorted {
		if i > 0 {
			if separator(item, sorted[i-1]) {
				buf.WriteString("\n\n")
			} else {
				buf.WriteString("\n")
			}
			for _, c := range item.comments {
				buf.WriteString(indent + c + "\n")
			}
			buf.WriteString(indent)
		} else {
			for _, c := range item.comments {
				buf.WriteString(c + "\n" + indent)
			}
		}
		buf.WriteString(string(item.text))
	}
}

func buildBlockFix(items []bodyItem, bodyStart *hcl.Pos) func(tflint.Fixer) error {
	return func(f tflint.Fixer) error {
		return applyFix(f, func() error {
			if len(items) < 2 {
				return nil
			}

			rich := attachComments(items, f, bodyStart)

			sorted := make([]richItem, len(rich))
			copy(sorted, rich)
			sort.SliceStable(sorted, func(i, j int) bool {
				if sorted[i].category != sorted[j].category {
					return sorted[i].category < sorted[j].category
				}
				return sorted[i].name < sorted[j].name
			})

			indent := strings.Repeat(" ", items[0].fullRange.Start.Column-1)
			for i := range sorted {
				sorted[i].text = f.TextAt(sorted[i].fullRange).Bytes
			}

			var buf strings.Builder
			writeItems(&buf, sorted, indent, func(item, prev richItem) bool {
				return needsBlankLineBefore(item.bodyItem, prev.bodyItem)
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

func (r *TerraformSortedArgumentsRule) checkBlock(runner tflint.Runner, block *hclsyntax.Block, src []byte) error {
	items := collectBodyItems(block.Body, src)
	bodyStart := block.OpenBraceRange.End
	hasViolations := blockHasViolations(items)
	fix := buildBlockFix(items, &bodyStart)

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
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		// Rule 2 — alphabetical order within the same category.
		if needsAlphaCheck(item, prev) && item.name < prev.name {
			msg := fmt.Sprintf(
				"argument %q is not sorted: it should come before %q",
				item.name, prev.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		// Rule 3 — blank line requirement.
		if needsBlankLineBefore(item, prev) && item.startLine <= prev.endLine+1 {
			msg := fmt.Sprintf(
				"missing blank line before %q (%s)",
				item.name, catLabel[item.category],
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}

		// Rule 4 — no blank line within the top-meta group (provider/count/for_each/source).
		if prev.category <= catSource && item.category <= catSource && item.startLine > prev.endLine+1 {
			msg := fmt.Sprintf(
				"unexpected blank line before %q: provider, count/for_each, and source should form a single unit",
				item.name,
			)
			if err := runner.EmitIssueWithFix(r, msg, item.nameRange, fix); err != nil {
				return err
			}
		}
	}

	if !hasViolations {
		for _, nested := range block.Body.Blocks {
			if err := r.checkBlock(runner, nested, src); err != nil {
				return err
			}
		}
	}

	return nil
}
