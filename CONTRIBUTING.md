# Contributing

We're happy to receive feature requests and PRs. If your change is
nontrivial, please open an
[issue](https://github.com/fastly/terraform-provider-fastly-beta/issues/new)
to discuss the idea and implementation strategy before submitting a
PR.

1. Fork the repository.

1. Create an `upstream` remote.
```bash
$ git remote add upstream git@github.com:fastly/terraform-provider-fastly-beta.git
```

1. Create a feature branch.

1. Make changes.

1. Write tests.

1. Validate your change via the steps documented [in the
   README](./README.md#testing).

1. Review the [documentation guide](./DOCUMENTATION.md) to ensure that
   you have properly documented your changes.

1. Open a pull request against `upstream main`. Note: once you have
   marked your PR as `Ready for Review` you should avoid 'force
   pushing' to the branch unless a reviewer asks you to do so.

1. Add an entry in `CHANGELOG.md` in the `UNRELEASED` section under
   the appropriate heading with a link to the PR.

1. Celebrate :tada:!

## Go version policy

The `go` directive in `go.mod` is the minimum toolchain for building the
provider, so it must not lag behind Go point releases that fix `cmd/go`
module-authentication vulnerabilities. CI and release workflows read the Go
version from `go.mod` (`go-version-file`), so bumping it upgrades them too.

The [`go-point-release`](./.github/workflows/go-point-release.yml) workflow
runs daily, compares `go.mod` against the latest stable release of the same
minor version from `go.dev/dl`, and opens a PR (with its CHANGELOG entry) when
a newer point release exists. Review and merge these PRs promptly. Moving to a
new Go minor version (for example 1.26 to 1.27) is not automated and remains a
manual change.
