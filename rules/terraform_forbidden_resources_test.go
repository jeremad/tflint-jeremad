package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

func TestTerraformForbiddenResources(t *testing.T) {
	rule := NewTerraformForbiddenResourcesRule()

	if rule.Severity() != tflint.ERROR {
		t.Fatalf("expected severity ERROR, got %v", rule.Severity())
	}

	cases := []struct {
		name   string
		config string
		issues helper.Issues
	}{
		{
			name: "allowed resource - no issues",
			config: `
resource "google_project_iam_member" "foo" {
  project = "p"
  role    = "roles/viewer"
  member  = "user:a@b.c"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "forbidden google_project_iam_policy",
			config: `
resource "google_project_iam_policy" "foo" {
  project     = "p"
  policy_data = "x"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `resource "google_project_iam_policy" is forbidden: google_project_iam_policy is authoritative and replaces the entire IAM policy on a project; use google_project_iam_binding or google_project_iam_member instead`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 2, Column: 10},
						End:      hcl.Pos{Line: 2, Column: 37},
					},
				},
			},
		},
		{
			name: "data source with forbidden name - no issues",
			config: `
data "google_project_iam_policy" "foo" {
  project = "p"
}
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
		})
	}
}
