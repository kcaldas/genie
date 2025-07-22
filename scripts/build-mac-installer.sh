#!/bin/bash
set -e

echo "Building macOS installer for Genie..."

# Check if we have a binary
if [ ! -f "dist/genie_darwin_arm64_v8.0/genie" ]; then
    echo "Error: No binary found. Run 'goreleaser release --snapshot --clean' first"
    exit 1
fi

# Create installer package directory
INSTALLER_DIR="dist/macos-installer"
mkdir -p "$INSTALLER_DIR/bin"
mkdir -p "$INSTALLER_DIR/scripts"

# Copy appropriate binary based on architecture
if [ "$(uname -m)" = "arm64" ]; then
    cp dist/genie_darwin_arm64_v8.0/genie "$INSTALLER_DIR/bin/"
else
    cp dist/genie_darwin_amd64_v1/genie "$INSTALLER_DIR/bin/"
fi

# Create post-install script
cat > "$INSTALLER_DIR/scripts/postinstall" << 'EOF'
#!/bin/bash
# Create symlink in /usr/local/bin
mkdir -p /usr/local/bin
ln -sf /Applications/Genie/bin/genie /usr/local/bin/genie

# Make executable
chmod +x /Applications/Genie/bin/genie

echo "Genie has been installed successfully!"
echo "You can now use 'genie' from your terminal."
exit 0
EOF

chmod +x "$INSTALLER_DIR/scripts/postinstall"

# Build the package
pkgbuild --root "$INSTALLER_DIR" \
         --identifier "com.kcaldas.genie" \
         --version "$(git describe --tags --abbrev=0 2>/dev/null || echo '0.1.0')" \
         --scripts "$INSTALLER_DIR/scripts" \
         --install-location "/Applications/Genie" \
         "dist/Genie-Installer.pkg"

echo "✅ Installer created: dist/Genie-Installer.pkg"

# Optional: Create a simple DMG with drag-to-install
echo "Creating DMG..."
mkdir -p dist/dmg
cp dist/Genie-Installer.pkg dist/dmg/
ln -s /Applications dist/dmg/Applications

hdiutil create -volname "Genie Installer" \
               -srcfolder dist/dmg \
               -ov -format UDZO \
               dist/Genie-Installer.dmg

rm -rf dist/dmg

echo "✅ DMG created: dist/Genie-Installer.dmg"
echo ""
echo "Distribution files ready:"
echo "  - PKG installer: dist/Genie-Installer.pkg"
echo "  - DMG installer: dist/Genie-Installer.dmg"