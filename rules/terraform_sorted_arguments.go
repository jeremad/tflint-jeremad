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

// extendToLineEnd advances pos to the end of its current line, dropping
// trailing whitespace. We use this on attribute expression-end positions so
// that an inline trailing comment (e.g. `foo = bar # note`) is captured as
// part of the attribute's text and travels with it during a reorder.
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
		exprEnd := extendToLineEnd(src, attr.Expr.Range().End)
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
//   - Any forward category transition requires a blank line, except within
//     the top-meta group (provider/count/for_each/source), which is one unit.
//   - The body of a `dynamic` block (the magic `content` block) does not
//     require a blank line before it on a forward transition.
//   - Consecutive complex variables (lists/maps) each require a blank line.
//   - Consecutive different-type nested blocks require a blank line. The
//     `dynamic` block always gets its own blank line, even between two
//     `dynamic` blocks (their content is intentionally distinct).
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

// replaceTextOrConflict wraps Fixer.ReplaceText so callers can detect a
// range conflict (the SDK returns an error when a fix range overlaps with
// an already-applied fix) via errors.Is(err, errFixConflict).
func replaceTextOrConflict(f tflint.Fixer, rng hcl.Range, text string) error {
	if err := f.ReplaceText(rng, text); err != nil {
		return errors.Join(errFixConflict, err)
	}
	return nil
}

// applyFix runs fn and converts a fix-range conflict into ErrFixNotSupported,
// so a single conflicting sub-fix downgrades the whole block fix gracefully
// instead of failing the rule.
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
	baseRule
}

func NewTerraformSortedArgumentsRule() *TerraformSortedArgumentsRule {
	return &TerraformSortedArgumentsRule{baseRule{name: "terraform_sorted_arguments"}}
}

func (r *TerraformSortedArgumentsRule) Check(runner tflint.Runner) error {
	return forEachFile(runner, func(file *hcl.File) error {
		return r.checkFile(runner, file)
	})
}

// argViolation describes a single ordering violation between two adjacent
// items in a body. Computed once per block so that the recursion-skip logic
// and the issue-emission loop share predicate evaluation.
type argViolation struct {
	item bodyItem
	msg  string
}

func argViolations(items []bodyItem) []argViolation {
	var out []argViolation
	for i := 1; i < len(items); i++ {
		prev, item := items[i-1], items[i]

		if item.category < prev.category {
			out = append(out, argViolation{item, fmt.Sprintf(
				"argument %q (%s) should come before %q (%s): %s",
				item.name, catLabel[item.category],
				prev.name, catLabel[prev.category],
				orderingHint,
			)})
		}
		if needsAlphaCheck(item, prev) && item.name < prev.name {
			out = append(out, argViolation{item, fmt.Sprintf(
				"argument %q is not sorted: it should come before %q",
				item.name, prev.name,
			)})
		}
		if needsBlankLineBefore(item, prev) && item.startLine <= prev.endLine+1 {
			out = append(out, argViolation{item, fmt.Sprintf(
				"missing blank line before %q (%s)",
				item.name, catLabel[item.category],
			)})
		}
		if prev.category <= catSource && item.category <= catSource && item.startLine > prev.endLine+1 {
			out = append(out, argViolation{item, fmt.Sprintf(
				"unexpected blank line before %q: provider, count/for_each, and source should form a single unit",
				item.name,
			)})
		}
	}
	return out
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

// extractCommentLines returns the comment lines found between two body
// items. The first split line — the remainder of the previous item's line
// — is skipped so that an inline trailing comment stays attached to the
// previous item rather than migrating to the next.
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
		} else if startsLineComment(trimmed) {
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
		if startsAnyComment(trimmed) {
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

// attachComments walks items in source order and pairs each item with the
// comment lines that appear before it. For the first item, the gap runs
// from bodyStart (the position right after the body's opening `{`) up to
// the item; we prepend a synthetic newline so that extractCommentLines's
// "skip first line" rule (which exists to keep inline comments with the
// previous item) does not eat the first real comment line.
func attachComments(items []bodyItem, f tflint.Fixer, bodyStart hcl.Pos) []richItem {
	rich := make([]richItem, len(items))
	for i, item := range items {
		gapStart := bodyStart
		if i > 0 {
			gapStart = items[i-1].fullRange.End
		}
		gapRange := hcl.Range{
			Filename: item.fullRange.Filename,
			Start:    gapStart,
			End:      item.fullRange.Start,
		}
		gap := string(f.TextAt(gapRange).Bytes)
		var comments []string
		if i == 0 {
			comments = extractCommentLines("\n" + gap)
		} else {
			comments = extractCommentLines(gap)
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

// buildBlockFix returns a Fixer callback that rewrites a body's items in
// canonical order. The output is intentionally not whitespace-perfect: the
// SDK runs hclwrite.Format on every plugin's changes before persisting,
// which re-indents and re-aligns `=` columns, so we leave alignment to the
// formatter and focus on getting the structural layout right.
func buildBlockFix(items []bodyItem, bodyStart hcl.Pos) func(tflint.Fixer) error {
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

func (r *TerraformSortedArgumentsRule) checkBlock(runner tflint.Runner, block *hclsyntax.Block, src []byte) error {
	items := collectBodyItems(block.Body, src)
	violations := argViolations(items)

	if len(violations) == 0 {
		// No violation here, so the block won't be reflowed. Recurse
		// into nested blocks; if a parent reflow were going to happen,
		// it would supersede any child fix anyway.
		for _, nested := range block.Body.Blocks {
			if err := r.checkBlock(runner, nested, src); err != nil {
				return err
			}
		}
		return nil
	}

	fix := buildBlockFix(items, block.OpenBraceRange.End)
	for _, v := range violations {
		if err := runner.EmitIssueWithFix(r, v.msg, v.item.nameRange, fix); err != nil {
			return err
		}
	}
	return nil
}
