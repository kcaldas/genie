# Release Checklist - v0.1.6

## Pre-Release

- [x] Security review complete
- [x] Documentation updated
- [x] GitHub templates and policies added
- [x] CI/CD workflows configured
- [x] Version updated in code (using ldflags)
- [x] Changelog updated
- [x] GoReleaser snapshot test

## Release Process

### 1. Version Update
```bash
# Update version in relevant files if needed
grep -r "0.0.0" . --exclude-dir=.git
```

### 2. Update Changelog
```bash
# Update CHANGELOG.md with release notes
# Follow Keep a Changelog format (https://keepachangelog.com)
```

### 3. Test Release Process
```bash
# Test GoReleaser snapshot
goreleaser release --snapshot --clean

# Build macOS installer (manual step)
./scripts/build-mac-installer.sh

# Test Docker build
docker build -f Dockerfile.local -t genie:test .
```

### 4. Create Release
```bash
# Tag and push
git tag v0.1.6
git push origin v0.1.6

# This triggers GitHub Actions release workflow
```

## Post-Release

- [ ] Verify GitHub release created
- [ ] Test downloads work
- [ ] Test Docker images
- [ ] Test Homebrew cask
- [ ] Announce release
- [ ] Update documentation links

