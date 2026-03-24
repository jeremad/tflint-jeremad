package rules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestTerraformSortedArguments(t *testing.T) {
	rule := NewTerraformSortedArgumentsRule()

	cases := []struct {
		name   string
		config string
		issues helper.Issues
	}{
		// ── Alphabetical ordering (simple scalars) ──────────────────────────────
		{
			name: "simple attrs sorted - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "simple attrs unsorted - reports issue",
			config: `
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "ami" is not sorted: it should come before "instance_type"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 6},
					},
				},
			},
		},

		// ── Category ordering: primitive before complex ──────────────────────────
		{
			name: "dict after simple with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  tags = { Name = "web" }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "dict before simple - category violation",
			config: `
resource "aws_instance" "web" {
  tags          = { Name = "web" }
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "instance_type" (primitive variable) should come before "tags" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
		},
		{
			name: "array before simple - category violation",
			config: `
resource "aws_instance" "web" {
  security_groups = ["sg-1"]
  ami             = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "ami" (primitive variable) should come before "security_groups" (complex variable (list/map)): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 6},
					},
				},
			},
		},

		// ── Blank line before complex/block items ───────────────────────────────
		{
			name: "dict without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami  = "ami-a1b2c3d4"
  tags = { Name = "web" }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "tags" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 7},
					},
				},
			},
		},
		{
			name: "array without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami             = "ami-a1b2c3d4"
  security_groups = ["sg-1"]
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "security_groups" (complex variable (list/map))`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 18},
					},
				},
			},
		},
		{
			name: "nested block without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "root_block_device" (nested block)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 20},
					},
				},
			},
		},
		{
			name: "nested block with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{},
		},

		// ── Alphabetical within complex category ────────────────────────────────
		{
			name: "two dicts unsorted - sort violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  tags      = { Name = "web" }

  metadata  = { key = "val" }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "metadata" is not sorted: it should come before "tags"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 7, Column: 3},
						End:      hcl.Pos{Line: 7, Column: 11},
					},
				},
			},
		},

		// ── Nested block body is also checked ───────────────────────────────────
		{
			name: "nested block args unsorted - reports issue inside nested block",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  root_block_device {
    volume_size           = 20
    delete_on_termination = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `argument "delete_on_termination" is not sorted: it should come before "volume_size"`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 7, Column: 5},
						End:      hcl.Pos{Line: 7, Column: 26},
					},
				},
			},
		},

		// ── Meta-argument: source for modules ───────────────────────────────────
		{
			name: "module source first with blank line - no issues",
			config: `
module "database" {
  source = "../modules/database"

  db_size = 10
  region  = "us-east-1"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "module source not first - category violation",
			config: `
module "database" {
  db_size = 10
  source  = "../modules/database"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "source" (source) should come before "db_size" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 9},
					},
				},
			},
		},
		{
			name: "module source first without blank line - blank line violation",
			config: `
module "database" {
  source  = "../modules/database"
  db_size = 10
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "db_size" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 10},
					},
				},
			},
		},

		// ── Meta-argument: for_each / count for resources ────────────────────────
		{
			name: "resource for_each first with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  for_each = toset(["a", "b"])

  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "resource for_each not first - category violation",
			config: `
resource "aws_instance" "web" {
  ami      = "ami-a1b2c3d4"
  for_each = toset(["a", "b"])
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "for_each" (instantiation meta-argument (count/for_each)) should come before "ami" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 11},
					},
				},
			},
		},
		{
			name: "resource for_each first without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  for_each      = toset(["a", "b"])
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "instance_type" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
		},
		{
			name: "data source count first with blank line - no issues",
			config: `
data "aws_ami" "latest" {
  count = 1

  most_recent = true
}
`,
			issues: helper.Issues{},
		},

		// ── Category 0: provider argument ───────────────────────────────────────
		{
			name: "provider argument first - no issues",
			config: `
resource "aws_instance" "web" {
  provider = aws.us_east

  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{},
		},
		{
			name: "provider argument after primitive - category violation",
			config: `
resource "aws_instance" "web" {
  ami      = "ami-a1b2c3d4"
  provider = aws.us_east
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "provider" (provider) should come before "ami" (primitive variable): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 11},
					},
				},
			},
		},
		{
			name: "provider without blank line before primitives - blank line violation",
			config: `
resource "aws_instance" "web" {
  provider      = aws.us_east
  instance_type = "t2.micro"
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "instance_type" (primitive variable)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 16},
					},
				},
			},
		},

		// ── Category 7: lifecycle meta-arguments at bottom ───────────────────────
		{
			name: "lifecycle at bottom with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "depends_on at bottom with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  depends_on = [aws_vpc.main]
}
`,
			issues: helper.Issues{},
		},
		{
			name: "lifecycle before primitives - category violation",
			config: `
resource "aws_instance" "web" {
  lifecycle {
    create_before_destroy = true
  }
  ami = "ami-a1b2c3d4"
}
`,
			issues: helper.Issues{
				{
					Rule: rule,
					Message: `argument "ami" (primitive variable) should come before "lifecycle" (lifecycle meta-argument): ` +
						orderingHint,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 6},
					},
				},
			},
		},
		{
			name: "lifecycle without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"
  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "lifecycle" (lifecycle meta-argument)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 4, Column: 3},
						End:      hcl.Pos{Line: 4, Column: 12},
					},
				},
			},
		},
		{
			name: "depends_on and lifecycle both at bottom - blank line between each",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  depends_on = [aws_vpc.main]

  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "depends_on and lifecycle without blank line between - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  depends_on = [aws_vpc.main]
  lifecycle {
    create_before_destroy = true
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "lifecycle" (lifecycle meta-argument)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 6, Column: 3},
						End:      hcl.Pos{Line: 6, Column: 12},
					},
				},
			},
		},

		// ── Consecutive same-type blocks: grouped without blank line ─────────────
		{
			name: "consecutive same-type blocks without blank line - no issues",
			config: `
resource "aws_autoscaling_group" "web" {
  ami = "ami-a1b2c3d4"

  tag {
    key   = "Name"
    value = "web"
  }
  tag {
    key   = "Env"
    value = "prod"
  }
}
`,
			issues: helper.Issues{},
		},
		{
			name: "consecutive different-type blocks without blank line - blank line violation",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  ebs_block_device {
    device_name = "/dev/sdb"
  }
  root_block_device {
    volume_size = 20
  }
}
`,
			issues: helper.Issues{
				{
					Rule:    rule,
					Message: `missing blank line before "root_block_device" (nested block)`,
					Range: hcl.Range{
						Filename: "main.tf",
						Start:    hcl.Pos{Line: 8, Column: 3},
						End:      hcl.Pos{Line: 8, Column: 20},
					},
				},
			},
		},
		{
			name: "consecutive different-type blocks with blank line - no issues",
			config: `
resource "aws_instance" "web" {
  ami = "ami-a1b2c3d4"

  ebs_block_device {
    device_name = "/dev/sdb"
  }

  root_block_device {
    volume_size = 20
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
		})
	}
}
