#!/bin/bash
set -e

echo "Building macOS installer for Genie..."

# Get version from git tag or default
VERSION=$(git describe --tags --exact-match 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo "0.1.0")
echo "Building version: $VERSION"

# Check if we have binaries
if [ ! -f "dist/genie_darwin_arm64_v8.0/genie" ] || [ ! -f "dist/genie_darwin_amd64_v1/genie" ]; then
    echo "Error: No binaries found. Run 'goreleaser release --snapshot --clean' first"
    exit 1
fi

# Function to build installer for a specific architecture
build_installer_for_arch() {
    local arch="$1"
    local binary_path="$2"
    local arch_name="$3"
    
    echo "Building installer for $arch_name ($arch)..."
    
    # Create installer package directory for this arch
    local installer_dir="dist/macos-installer-$arch"
    mkdir -p "$installer_dir/bin"
    mkdir -p "$installer_dir/scripts"
    
    # Copy the binary for this architecture
    cp "$binary_path" "$installer_dir/bin/"
    
    # Create post-install script
    cat > "$installer_dir/scripts/postinstall" << EOF
#!/bin/bash
# Create symlink in /usr/local/bin
mkdir -p /usr/local/bin
ln -sf /Applications/Genie/bin/genie /usr/local/bin/genie

# Make executable
chmod +x /Applications/Genie/bin/genie

echo "Genie $VERSION ($arch_name) has been installed successfully!"
echo "You can now use 'genie' from your terminal."
echo "Run 'genie --version' to verify the installation."
exit 0
EOF

    chmod +x "$installer_dir/scripts/postinstall"

    # Build the package with version and architecture in filename
    local pkg_name="dist/Genie-${VERSION}-${arch}-Installer.pkg"
    pkgbuild --root "$installer_dir" \
             --identifier "com.kcaldas.genie" \
             --version "${VERSION#v}" \
             --scripts "$installer_dir/scripts" \
             --install-location "/Applications/Genie" \
             "$pkg_name"

    echo "✅ Installer created: $pkg_name"

    # Create a DMG with drag-to-install
    echo "Creating DMG for $arch_name..."
    mkdir -p "dist/dmg-$arch"
    cp "$pkg_name" "dist/dmg-$arch/"
    ln -s /Applications "dist/dmg-$arch/Applications"

    local dmg_name="dist/Genie-${VERSION}-${arch}-Installer.dmg"
    hdiutil create -volname "Genie ${VERSION} Installer ($arch_name)" \
                   -srcfolder "dist/dmg-$arch" \
                   -ov -format UDZO \
                   "$dmg_name"

    rm -rf "dist/dmg-$arch"
    echo "✅ DMG created: $dmg_name"
}

# Build installers for both architectures
build_installer_for_arch "arm64" "dist/genie_darwin_arm64_v8.0/genie" "Apple Silicon"
build_installer_for_arch "amd64" "dist/genie_darwin_amd64_v1/genie" "Intel"

echo ""
echo "Distribution files ready:"
echo "  - PKG installer (Apple Silicon): dist/Genie-${VERSION}-arm64-Installer.pkg"
echo "  - DMG installer (Apple Silicon): dist/Genie-${VERSION}-arm64-Installer.dmg"
echo "  - PKG installer (Intel): dist/Genie-${VERSION}-amd64-Installer.pkg"
echo "  - DMG installer (Intel): dist/Genie-${VERSION}-amd64-Installer.dmg"