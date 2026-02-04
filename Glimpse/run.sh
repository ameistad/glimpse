#!/bin/bash
set -e

cd "$(dirname "$0")"

pkill -x Glimpse 2>/dev/null || true

swift build -c release \
    -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker "$(pwd)/Info.plist"

mkdir -p Glimpse.app/Contents/MacOS
mkdir -p Glimpse.app/Contents/Resources

cp .build/release/Glimpse Glimpse.app/Contents/MacOS/Glimpse
cp Info.plist Glimpse.app/Contents/Info.plist
cp AppIcon.icns Glimpse.app/Contents/Resources/AppIcon.icns

open Glimpse.app
