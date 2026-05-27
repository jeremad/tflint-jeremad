# tflint-ruleset-jeremad

A [TFLint](https://github.com/terraform-linters/tflint) ruleset plugin that enforces a canonical argument ordering and comment style in Terraform configuration files.

## Rules

| Rule | Description | Severity |
|------|-------------|----------|
| [terraform_sorted_arguments](rules/terraform_sorted_arguments.go) | Enforces canonical argument ordering within blocks | Warning |
| [terraform_sorted_variables](rules/terraform_sorted_variables.go) | Enforces canonical ordering inside `variable` blocks (`type → default → description`, then other fields alphabetically, then `validation`) | Warning |
| [terraform_block_comment_format](rules/terraform_block_comment_format.go) | Enforces multi-line block-comment formatting (`/*` on its own line, aligned `*` body, `*/` on its own line) | Warning |
| [terraform_comment_style](rules/terraform_comment_style.go) | Converts long runs of consecutive `#` / `//` line comments into a single block comment | Warning |
| [terraform_single_line_comment_style](rules/terraform_single_line_comment_style.go) | Rewrites single-line `/* … */` and `// …` comments to `# …` | Warning |
| [terraform_no_empty_comment](rules/terraform_no_empty_comment.go) | Removes empty `#`, `//`, `/**/`, and `/* */` comments | Warning |
| [terraform_forbidden_resources](rules/terraform_forbidden_resources.go) | Forbids use of disallowed `resource` types (currently `google_project_iam_policy`, `google_organization_iam_policy`) | Error |

### `terraform_sorted_arguments`

Arguments inside a block must appear in the following order, top to bottom:

1. **`provider`** — the provider alias argument
2. **Instantiation meta-arguments** — `count`, `for_each`
3. **`source`** — module source (modules only)
4. **Primitive variables** — booleans, numbers, strings, references, function calls; sorted alphabetically within this group
5. **Complex variables** — lists (`[…]`), maps (`{…}`), `for` expressions, multi-line expressions (heredocs), and calls to complex-producing functions (`compact`, `concat`, `jsonencode`, `merge`, `setsubtract`, `templatefile`, `toset`); sorted alphabetically, each separated from the previous group by a blank line
6. **Nested blocks** — HCL sub-blocks; separated by a blank line, sorted alphabetically (consecutive blocks of the same type may be grouped without a blank line between them)
7. **Lifecycle meta-arguments** — `lifecycle`, `depends_on`; each preceded by a blank line

### `terraform_sorted_variables`

Attributes inside a `variable` block must appear in the following order:

1. **`type`**, **`default`**, **`description`** — in this fixed order, no blank lines between them
2. **Other fields** — `const`, `deprecated`, `ephemeral`, `nullable`, `sensitive` — sorted alphabetically, no blank lines
3. **`validation`** block — always last, preceded by a blank line

### `terraform_forbidden_resources`

Flags `resource` blocks whose type is on a deny-list. Currently forbidden:

- **`google_project_iam_policy`** — authoritative; replaces the entire IAM policy on a project. Use `google_project_iam_binding` or `google_project_iam_member` instead.
- **`google_organization_iam_policy`** — authoritative; overwrites the entire IAM policy on an organization (including default policies). Use `google_organization_iam_binding` or `google_organization_iam_member` instead.

This rule has no auto-fix: removing the resource would silently drop infrastructure, so the change must be made deliberately.

### How the comment rules interact

The four comment rules cooperate; they do not oscillate, but the chain is worth knowing:

- `terraform_no_empty_comment` removes empty comments first.
- `terraform_single_line_comment_style` rewrites single-line `/* … */` and `// …` to `# …`.
- `terraform_comment_style` converts long runs of consecutive `#`/`//` line comments (≥ 3 lines, or ≥ 2 if any are `//`) into a single multi-line block comment.
- `terraform_block_comment_format` enforces the canonical multi-line block-comment shape on the result.

The single-line rule only triggers on one-line `/* … */` forms, so a multi-line block created by `terraform_comment_style` is not converted back. Net effect: short stretches of comments stay as `#`, longer ones become block comments.

## Installation

### Via `tflint --init` (recommended)

Add the following to your `.tflint.hcl`:

```hcl
plugin "jeremad" {
  enabled = true
  version = "0.1.0"
  source  = "github.com/jeremad/tflint-jeremad"
}
```

Then run:

```sh
tflint --init
```

### Manual

Download the binary for your platform from the [releases page](https://github.com/jeremad/tflint-jeremad/releases), place it in `~/.tflint.d/plugins/`, and make it executable.

### From source

```sh
git clone https://github.com/jeremad/tflint-jeremad.git
cd tflint-jeremad
make install
```

## Usage

```sh
tflint --enable-plugin=jeremad
```

Or with the `.tflint.hcl` config above, simply run:

```sh
tflint
```

## Example

The following block will trigger violations:

```hcl
resource "aws_instance" "web" {
  tags          = { Name = "web" }   # complex variable before primitives
  instance_type = "t2.micro"
  ami           = "ami-a1b2c3d4"     # primitives not sorted alphabetically
  lifecycle {                        # no blank line before lifecycle block
    create_before_destroy = true
  }
}
```

Corrected version:

```hcl
resource "aws_instance" "web" {
  ami           = "ami-a1b2c3d4"
  instance_type = "t2.micro"

  tags = { Name = "web" }

  lifecycle {
    create_before_destroy = true
  }
}
```

## Development

```sh
make test    # run tests
make build   # build the binary
make install # install to ~/.tflint.d/plugins
```

## Release

Releases are automated via GitHub Actions and [GoReleaser](https://goreleaser.com). Push a version tag to trigger a release:

```sh
git tag v0.1.1
git push origin v0.1.1
```

Binaries for all supported platforms will be built, signed, and published automatically.
