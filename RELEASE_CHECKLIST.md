# Release Checklist

Use this checklist for each new release. Replace `vX.X.X` with the actual version number.

## Pre-Release

- [ ] Security review complete
- [ ] Documentation updated  
- [ ] Version updated in code (using ldflags)
- [ ] Changelog updated with git log since last tag
- [ ] GoReleaser snapshot test passes

## Release Process

### 1. Update Changelog
```bash
# Generate changelog entries from git log since last tag
git log --pretty=format:"* %h %s" $(git describe --tags --abbrev=0)..HEAD

# Update CHANGELOG.md with release notes
# Follow Keep a Changelog format (https://keepachangelog.com)
```

### 2. Commit Changes
```bash
# Commit changelog updates
git add CHANGELOG.md
git commit -m "docs: Update changelog for vX.X.X"
```

### 3. Test Release Process
```bash
# Test GoReleaser snapshot build
goreleaser release --snapshot --clean

# Test macOS installer build (requires GoReleaser dist/ output)
./scripts/build-mac-installer.sh
```

### 4. Create and Push Tag
```bash
# Create and push tag (triggers GitHub Actions)
git tag vX.X.X
git push origin vX.X.X
```

### 5. Wait for GitHub Actions
- Monitor GitHub Actions workflow completion
- Verify draft release is created with all platform binaries

### 6. Generate macOS Installers
```bash
# Generate architecture-specific macOS installers
./scripts/build-mac-installer.sh
```

### 7. Upload macOS Installers
```bash
# Upload PKG and DMG files to GitHub release
gh release upload vX.X.X dist/Genie-vX.X.X-*.pkg dist/Genie-vX.X.X-*.dmg
```

### 8. Publish Release
```bash
# Remove draft status to publish release
gh release edit vX.X.X --draft=false
```

## Post-Release Verification

- [ ] Verify GitHub release is published
- [ ] Test binary downloads work
- [ ] Test macOS installers work
- [ ] Test Docker images
- [ ] Test self-update functionality
- [ ] Update version number for next development cycle
- [ ] Announce release (if appropriate)

