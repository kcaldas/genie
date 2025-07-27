# Release Command

Follow the release checklist to create a new version of Genie.

## Steps:

1. Review the RELEASE_CHECKLIST.md file in the project root
2. Update version number (handled by GoReleaser ldflags)
3. Update CHANGELOG.md with new features since last release
4. Test the release process locally
5. Create and push the git tag
6. Generate macOS installers
7. Upload installers to GitHub release
8. Publish the release

## Usage:

This command serves as a reminder to follow the complete release process. Please refer to RELEASE_CHECKLIST.md for detailed instructions.

The typical flow is:
```bash
# 1. Update changelog first
git add CHANGELOG.md
git commit -m "docs: Update changelog for vX.X.X"

# 2. Test release process
goreleaser release --snapshot --clean

# 3. Create and push tag (triggers GitHub Actions)
git tag vX.X.X
git push origin vX.X.X

# 4. Generate macOS installers after GitHub Actions completes
./scripts/build-mac-installer.sh

# 5. Upload installers to GitHub release
gh release upload vX.X.X dist/Genie-vX.X.X-*.pkg dist/Genie-vX.X.X-*.dmg

# 6. Publish the release
gh release edit vX.X.X --draft=false
```

Replace X.X.X with the actual version number.