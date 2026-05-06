package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformSingleLineCommentStyle(t *testing.T) {
	rule := NewTerraformSingleLineCommentStyleRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		{
			name: "hash comment - no issues",
			config: `
# This is fine
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "multi-line block comment - no issues",
			config: `
/*
 * Line 1
 * Line 2
 */
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "single-line block comment - violation",
			config: `
/* This is a comment */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "single-line block comments should use # syntax",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
# This is a comment
locals {}
`,
		},
		{
			name: "indented single-line block comment - preserves indentation",
			config: `
resource "example" "test" {
  /* This is a comment */
  name = "test"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "single-line block comments should use # syntax",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 3, Column: 1},
						End:      hcl.Pos{Line: 4, Column: 1},
					},
				},
			},
			fixed: `
resource "example" "test" {
  # This is a comment
  name = "test"
}
`,
		},
		{
			name: "empty single-line block comment - skipped by this rule",
			config: `
/**/
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "double-slash comment - violation",
			config: `
// This is a comment
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "single-line // comments should use # syntax",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
# This is a comment
locals {}
`,
		},
		{
			name: "indented double-slash comment - preserves indentation",
			config: `
resource "example" "test" {
  // This is a comment
  name = "test"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "single-line // comments should use # syntax",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 3, Column: 1},
						End:      hcl.Pos{Line: 4, Column: 1},
					},
				},
			},
			fixed: `
resource "example" "test" {
  # This is a comment
  name = "test"
}
`,
		},
		{
			name: "empty double-slash comment - skipped by this rule",
			config: `
//
locals {}
`,
			issues: helper.Issues{},
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
