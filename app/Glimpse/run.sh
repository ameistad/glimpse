#!/bin/bash
set -e

cd "$(dirname "$0")"

pkill -x Glimpse 2>/dev/null || true
swift build -c release
cp .build/release/Glimpse Glimpse.app/Contents/MacOS/Glimpse
open Glimpse.app
