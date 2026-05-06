package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformBlockCommentFormat(t *testing.T) {
	rule := NewTerraformBlockCommentFormatRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		{
			name: "valid block comment - no issues",
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
			name: "/** opening - violation",
			config: `
/**
 * Line 1
 * Line 2
 */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment should start with /* not /**",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
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
			name: "content on opening line - violation",
			config: `
/* Line 1
 * Line 2
 */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment opening /* should be on its own line",
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
 */
locals {}
`,
		},
		{
			name: "misaligned stars - violation",
			config: `
/*
 * Line 1
  * Line 2
 */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment * characters are not aligned",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
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
			name: "content before closing */ - violation",
			config: `
/*
 * Line 1
 * Line 2
 something */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment closing */ should be on its own line",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Line 1
 * Line 2
 * something
 */
locals {}
`,
		},
		{
			name: "/** with content - violation",
			config: `
/**
  * The workload identity pool provider.
  * It describes the relationship between GCP and GitLab.
  */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment should start with /* not /**",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 6, Column: 1},
					},
				},
			},
			fixed: `
/*
 * The workload identity pool provider.
 * It describes the relationship between GCP and GitLab.
 */
locals {}
`,
		},
		{
			name: "indented block comment - preserves indentation",
			config: `
resource "example" "test" {
  /**
    * Line 1
    * Line 2
    */
  name = "test"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment should start with /* not /**",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 3, Column: 1},
						End:      hcl.Pos{Line: 7, Column: 1},
					},
				},
			},
			fixed: `
resource "example" "test" {
  /*
   * Line 1
   * Line 2
   */
  name = "test"
}
`,
		},
		{
			name: "single line block comment - no issues",
			config: `
/* single line */
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "trailing empty line before closing - violation",
			config: `
/*
 * Line 1
 * Line 2
 *
 */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment should not have trailing empty lines before */",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 7, Column: 1},
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
			name: "empty line in middle preserved - no issues",
			config: `
/*
 * Line 1
 *
 * Line 2
 */
locals {}
`,
			issues: helper.Issues{},
		},
		{
			name: "/** with trailing empty line and middle empty line",
			config: `
/**
 * Line 1
 *
 * Line 2
 *
 */
locals {}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: "block comment should start with /* not /**",
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 1},
						End:      hcl.Pos{Line: 8, Column: 1},
					},
				},
			},
			fixed: `
/*
 * Line 1
 *
 * Line 2
 */
locals {}
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
