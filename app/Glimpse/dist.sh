#!/bin/bash
set -e

cd "$(dirname "$0")"

APP_NAME="Glimpse"
APP_BUNDLE="${APP_NAME}.app"
DMG_NAME="${APP_NAME}.dmg"
VERSION=$(grep CFBundleShortVersionString Info.plist -A1 | grep string | sed 's/.*<string>\(.*\)<\/string>.*/\1/')

echo "Building ${APP_NAME} v${VERSION}..."
echo ""

# Build release binary
swift build -c release

# Assemble app bundle
rm -rf "${APP_BUNDLE}"
mkdir -p "${APP_BUNDLE}/Contents/MacOS"
mkdir -p "${APP_BUNDLE}/Contents/Resources"

cp .build/release/${APP_NAME} "${APP_BUNDLE}/Contents/MacOS/${APP_NAME}"
cp Info.plist "${APP_BUNDLE}/Contents/Info.plist"
cp AppIcon.icns "${APP_BUNDLE}/Contents/Resources/AppIcon.icns"

# Ad-hoc code sign
echo "Code signing (ad-hoc)..."
codesign --force --deep --sign - "${APP_BUNDLE}"

echo "Verifying signature..."
codesign --verify --verbose "${APP_BUNDLE}"

# Create DMG
echo ""
echo "Creating DMG..."
rm -f "${DMG_NAME}"

STAGING_DIR=$(mktemp -d)
cp -R "${APP_BUNDLE}" "${STAGING_DIR}/"
ln -s /Applications "${STAGING_DIR}/Applications"

hdiutil create -volname "${APP_NAME}" \
    -srcfolder "${STAGING_DIR}" \
    -ov -format UDZO \
    "${DMG_NAME}"

rm -rf "${STAGING_DIR}"

echo ""
echo "Done! Distribution files:"
echo "  App:  ${APP_BUNDLE}"
echo "  DMG:  ${DMG_NAME}"
echo ""
echo "To install: Open ${DMG_NAME} and drag ${APP_NAME} to Applications."
echo ""
echo "Note: Since the app is ad-hoc signed, recipients will need to"
echo "right-click > Open on first launch to bypass Gatekeeper."
