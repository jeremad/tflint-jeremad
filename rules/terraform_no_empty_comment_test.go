package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformNoEmptyComment(t *testing.T) {
	rule := NewTerraformNoEmptyCommentRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		{
			name: "comment with content - no issues",
			config: `
# This is fine
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "empty hash comment - violation",
			config: `
#
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "empty comments should be removed",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
locals {}
`,
		},
		{
			name: "empty double-slash comment - violation",
			config: `
//
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "empty comments should be removed",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
locals {}
`,
		},
		{
			name: "empty block comment - violation",
			config: `
/**/
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "empty comments should be removed",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
locals {}
`,
		},
		{
			name: "empty block comment with space - violation",
			config: `
/* */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "empty comments should be removed",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 3, Column: 1},
					},
				},
			},
			fixed: `
locals {}
`,
		},
		{
			name: "adjacent empty comments coalesce into one issue",
			config: `
#
//
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "empty comments should be removed",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 4, Column: 1},
					},
				},
			},
			fixed: `
locals {}
`,
		},
		{
			name: "block comment with content - no issues",
			config: `
/* something */
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "multi-line block comment - no issues",
			config: `
/*
 * content
 */
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
