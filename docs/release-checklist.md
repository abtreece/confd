# Release Checklist

This project uses [GoReleaser](https://goreleaser.com/) to automate releases. Follow these steps to cut a new release:

## Prerequisites

- GoReleaser installed (`brew install goreleaser` on macOS)
- Push access to the repository
- All CI checks passing on main branch

## Release Process

### 1. Prepare the Release

Ensure the codebase is ready:
```bash
# Run tests
make test

# Run linter
make lint

# Test build locally
make build
```

### 2. Update Version

Update the version in `cmd/confd/version.go`:
```go
const Version = "0.XX.0"
```

**Important**: The version string must exactly match the tag (without the `v` prefix).

### 3. Update Installation Documentation

Update version references in `docs/installation.md`:
- Binary download URLs
- Docker ARG CONFD_VERSION
- Multi-stage build ARG CONFD_VERSION

Example:
```bash
# Find and replace version
sed -i '' 's/v0.30.0/v0.31.0/g' docs/installation.md
sed -i '' 's/0.30.0/0.31.0/g' docs/installation.md
```

### 4. Commit Version Changes

```bash
git add cmd/confd/version.go docs/installation.md
git commit -m "chore: bump version to 0.XX.0"
git push origin main
```

### 5. Create and Push Tag

```bash
# Create annotated tag
git tag -a v0.XX.0 -m "Release v0.XX.0"

# Push tag (this triggers GoReleaser via GitHub Actions)
git push origin v0.XX.0
```

### 6. Monitor Release

GoReleaser will automatically:
- Build binaries for all platforms (darwin, linux, windows)
- Create archives (.tar.gz for unix, .zip for windows)
- Generate checksums
- Create GitHub release with auto-generated changelog
- Upload all artifacts to the release

Monitor the GitHub Actions workflow at:
https://github.com/abtreece/confd/actions

### 7. Verify Release

Once complete:
1. Visit https://github.com/abtreece/confd/releases
2. Verify the new release is published
3. Check that all binaries are attached
4. Review the auto-generated changelog

## Release Candidate Workflow

For significant releases, use release candidates to allow testing before the final release:

### RC Process

```bash
# 1. Update version.go to RC version
# Edit cmd/confd/version.go: const Version = "0.40.0-rc.1"

# 2. Update docs/installation.md with RC version

# 3. Commit and tag
git add cmd/confd/version.go docs/installation.md
git commit -m "chore: bump version to 0.40.0-rc.1"
git tag -a v0.40.0-rc.1 -m "v0.40.0-rc.1"
git push origin main v0.40.0-rc.1

# 4. If issues are found, fix them, then release rc.2
git add cmd/confd/version.go docs/installation.md
git commit -m "chore: bump version to 0.40.0-rc.2"
git tag -a v0.40.0-rc.2 -m "v0.40.0-rc.2"
git push origin main v0.40.0-rc.2

# 5. When stable, release final version
git add cmd/confd/version.go docs/installation.md
git commit -m "chore: bump version to 0.40.0"
git tag -a v0.40.0 -m "v0.40.0"
git push origin main v0.40.0
```

### When to Use RCs

- Major version bumps
- Significant new features
- Breaking changes
- Large refactors

### RC vs Standard Release

- **RC tags** (e.g., `v0.40.0-rc.1`) create pre-release builds marked as "Pre-release" on GitHub
- **Final tags** (e.g., `v0.40.0`) create production releases

## Manual Release (If Needed)

If you need to release manually without CI:

```bash
# Clean release
make release

# Snapshot release (for testing)
make snapshot
```

This will create binaries in `dist/` directory.

## Troubleshooting

**Issue**: GoReleaser fails with validation errors
- **Solution**: Run `goreleaser check` locally to validate configuration

**Issue**: Release artifacts missing
- **Solution**: Check `.goreleaser.yml` configuration

**Issue**: Changelog not generating correctly
- **Solution**: Ensure commits follow [Conventional Commits](https://www.conventionalcommits.org/) format

## Release Notes

GoReleaser automatically generates release notes from commit messages. For better release notes:
- Use conventional commit format: `feat:`, `fix:`, `docs:`, `chore:`, etc.
- Write descriptive commit messages
- Reference issues/PRs in commits: `fixes #123`

Commits with `^docs:` and `^test:` prefixes are automatically excluded from changelogs (see `.goreleaser.yml`).
