# Release Process

## Prerequisites

For security we sign tags. To be able to sign tags you need to tell Git which key you would like to use. Please follow these
[steps](https://docs.github.com/en/authentication/managing-commit-signature-verification/telling-git-about-your-signing-key) to
tell Git about your signing key.

## Steps

1. Merge all PRs intended for the release.
1. Rebase latest remote main branch locally (`git pull --rebase origin main`).
1. Update the `version` constraint in `examples/provider/provider.tf` to the new release version.
1. Ensure the code is release-ready (`make release-check`), which runs the build, lint, baseline test suite, and generates/validates docs. Requires `FASTLY_API_TOKEN` to be set, since the baseline test suite makes real API calls.
1. Open a new PR to update CHANGELOG ([example](https://github.com/fastly/terraform-provider-fastly/pull/498/files)).
    - We utilize [Semantic Versioning](https://semver.org/) and only include relevant/significant changes within the CHANGELOG.
1. 🚨 Ensure any _removals_ are considered a BREAKING CHANGE and must be published in a major release.
1. Merge CHANGELOG.
1. Rebase latest remote main branch locally (`git pull --rebase origin main`).
1. Create a new signed tag: `tag=vX.Y.Z && git tag -s $tag -m $tag && git push origin $tag`.
    - Triggers a [GitHub Action](https://github.com/fastly/terraform-provider-fastly-beta/blob/main/.github/workflows/release.yml) that produces a 'draft' release.
1. Copy/paste CHANGELOG into the [draft release](https://github.com/fastly/terraform-provider-fastly-beta/releases).
1. Publish draft release.
    - Triggers a [GitHub Webhook](https://github.com/fastly/terraform-provider-fastly-beta/settings/hooks) that produces a release on the [terraform registry](https://registry.terraform.io/providers/fastly/fastly-beta/latest).
