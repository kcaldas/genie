# gon configuration for macOS code signing and notarization
# https://github.com/mitchellh/gon

source = ["./dist/genie_darwin_amd64_v1/genie", "./dist/genie_darwin_arm64_v8.0/genie"]
bundle_id = "com.kcaldas.genie"

# Code signing
sign {
  application_identity = "Developer ID Application: Your Name (TEAM_ID)"
  entitlements_file = "./entitlements.plist"
}

# DMG creation
dmg {
  output_path = "./dist/Genie.dmg"
  volume_name = "Genie"
}

# Notarization (optional, requires Apple Developer account)
# notarize {
#   path = "./dist/Genie.dmg"
#   bundle_id = "com.kcaldas.genie"
#   staple = true
# }

# Zip for distribution
zip {
  output_path = "./dist/Genie-macOS.zip"
}