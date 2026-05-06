package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformCommentStyle(t *testing.T) {
	rule := NewTerraformCommentStyleRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		{
			name: "single line comment - no issues",
			config: `
# This is fine
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "block comment - no issues",
			config: `
/*
  Line 1
  Line 2
*/
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "two consecutive line comments - no issues",
			config: `
# Line 1
# Line 2
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "three consecutive line comments - violation",
			config: `
# Line 1
# Line 2
# Line 3
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 5, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Line 1
 * Line 2
 * Line 3
 */
locals {}
`,
		},
		{
			name: "two consecutive double-slash comments - violation",
			config: `
// Line 1
// Line 2
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 4, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Line 1
 * Line 2
 */
locals {}
`,
		},
		{
			name: "three consecutive double-slash comments - violation",
			config: `
// Line 1
// Line 2
// Line 3
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 5, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Line 1
 * Line 2
 * Line 3
 */
locals {}
`,
		},
		{
			name: "non-consecutive line comments - no issues",
			config: `
# Line 1
locals {}
# Line 2
resource "example" "test" {}
`,
			issues: helper.Issues{},
		},
		{
			name: "two separate groups of two comments - no issues",
			config: `
# Group 1 line 1
# Group 1 line 2
locals {}
# Group 2 line 1
# Group 2 line 2
resource "example" "test" {}
`,
			issues: helper.Issues{},
		},
		{
			name: "two separate groups of three comments - two violations",
			config: `
# Group 1 line 1
# Group 1 line 2
# Group 1 line 3
locals {}
# Group 2 line 1
# Group 2 line 2
# Group 2 line 3
resource "example" "test" {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 5, Column: 1},
					},
				},
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 1},
						End:      hcl.Pos{Line: 9, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Group 1 line 1
 * Group 1 line 2
 * Group 1 line 3
 */
locals {}
/*
 * Group 2 line 1
 * Group 2 line 2
 * Group 2 line 3
 */
resource "example" "test" {}
`,
		},
		{
			name: "three consecutive comments at end of file - violation",
			config: `
locals {}
# Line 1
# Line 2
# Line 3
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 3, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
					},
				},
			},
			fixed: `
locals {}
/*
 * Line 1
 * Line 2
 * Line 3
 */
`,
		},
		{
			name: "indented three consecutive comments - preserves indentation",
			config: `
resource "example" "test" {
  # Line 1
  # Line 2
  # Line 3
  name = "test"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "consecutive line comments should use block comment syntax (/* ... */)",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 3, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
					},
				},
			},
			fixed: `
resource "example" "test" {
  /*
   * Line 1
   * Line 2
   * Line 3
   */
  name = "test"
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := helper.TestRunner(t, map[string]string{"main.tf": tc.config})
			if err := rule.Check(runner); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			helper.AssertIssues(t, tc.issues, runner.Issues)

			if len(tc.issues) > 0 && tc.fixed == "" {
				t.Fatal("test case reports issues but has no 'fixed' assertion")
			}
			if tc.fixed != "" {
				got := string(runner.Changes()["main.tf"])
				if got != tc.fixed {
					t.Errorf("fix mismatch:\nwant:\n%s\ngot:\n%s", tc.fixed, got)
				}
			}
		})
	}
}
