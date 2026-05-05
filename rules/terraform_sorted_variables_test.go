package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformSortedVariables(t *testing.T) {
	rule := NewTerraformSortedVariablesRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
		fixed  string
	}{
		{
			name: "correct order type then default - no issues",
			config: `
variable "name" {
  type    = string
  default = "value"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "correct order type default description - no issues",
			config: `
variable "name" {
  type        = string
  default     = "value"
  description = "A variable"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "type only - no issues",
			config: `
variable "name" {
  type = string
}
`,
			issues: helper.Issues{},
		},
		{
			name: "default before type - ordering violation",
			config: `
variable "name" {
  default = "value"
  type    = string
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "type" should come before "default" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
variable "name" {
  type    = string
  default = "value"
}
`,
		},
		{
			name: "description before default - ordering violation",
			config: `
variable "name" {
  type        = string
  description = "A variable"
  default     = "value"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "default" should come before "description" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 10},
					},
				},
			},
			fixed: `
variable "name" {
  type        = string
  default     = "value"
  description = "A variable"
}
`,
		},
		{
			name: "description before type - ordering violation",
			config: `
variable "name" {
  description = "A variable"
  type        = string
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "type" should come before "description" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
variable "name" {
  type        = string
  description = "A variable"
}
`,
		},
		{
			name: "blank line between type and default - blank line violation",
			config: `
variable "name" {
  type = string

  default = "value"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `unexpected blank line before "default": variable block attributes should not be separated by blank lines`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 10},
					},
				},
			},
			fixed: `
variable "name" {
  type    = string
  default = "value"
}
`,
		},
		{
			name: "all three reversed - multiple violations",
			config: `
variable "name" {
  description = "A variable"
  default     = "value"
  type        = string
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "default" should come before "description" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 10},
					},
				},
				{
					Rule:    rule,
					Message: `argument "type" should come before "default" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 5, Column: 3},
						End:      hcl.Pos{Line: 5, Column: 7},
					},
				},
			},
			fixed: `
variable "name" {
  type        = string
  default     = "value"
  description = "A variable"
}
`,
		},
		{
			name: "non-variable block ignored - no issues",
			config: `
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "default is a list with correct order - no issues",
			config: `
variable "allowed_to_push" {
  type    = list(number)
  default = []
}
`,
			issues: helper.Issues{},
		},
		{
			name: "default is a list before type - ordering violation",
			config: `
variable "allowed_to_push" {
  default = []
  type    = list(number)
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "type" should come before "default" in variable blocks (required order: type → default → description)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
			fixed: `
variable "allowed_to_push" {
  type    = list(number)
  default = []
}
`,
		},
		{
			name: "comment between type and default - no blank line violation",
			config: `
variable "max_clients" {
  type = number
  # Change here must be sync in alerts
  default     = 5000
  description = "Maximum number of clients"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "blank line and comment between type and default - blank line violation",
			config: `
variable "max_clients" {
  type = number

  # Change here must be sync in alerts
  default     = 5000
  description = "Maximum number of clients"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `unexpected blank line before "default": variable block attributes should not be separated by blank lines`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 10},
					},
				},
			},
			fixed: `
variable "max_clients" {
  type = number
  # Change here must be sync in alerts
  default     = 5000
  description = "Maximum number of clients"
}
`,
		},
		{
			name: "variable with validation block - no issues",
			config: `
variable "name" {
  type    = string
  default = "value"

  validation {
    condition     = length(var.name) > 0
    error_message = "Must not be empty"
  }
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
