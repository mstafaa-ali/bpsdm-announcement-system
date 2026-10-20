#!/bin/bash
set -e

echo "================================================================"
echo "  Building Standalone Windows Binary for Smart Office Announcer"
echo "================================================================"

OUTPUT_DIR="Release_App_Go"
mkdir -p "$OUTPUT_DIR"

echo "[1/3] Cross-compiling for Windows (amd64, pure Go, CGO_ENABLED=0)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H=windowsgui" -o "$OUTPUT_DIR/SmartOfficeAnnouncer.exe" .

echo "[2/3] Checking UPX compression..."
if command -v upx >/dev/null 2>&1; then
    echo "  Applying UPX LZMA ultra-compression..."
    upx --best --lzma "$OUTPUT_DIR/SmartOfficeAnnouncer.exe"
else
    echo "  UPX not found, skipping compression (binary is already compact)."
fi

echo "[3/3] Copying runtime media and configuration..."
cp -f config.json "$OUTPUT_DIR/" 2>/dev/null || true
if [ -d "media" ]; then
    mkdir -p "$OUTPUT_DIR/media"
    cp -Rf media/* "$OUTPUT_DIR/media/" 2>/dev/null || true
fi

echo ""
echo "================================================================"
echo "  [SUCCESS] Portable Windows release ready in folder:"
echo "  -> $OUTPUT_DIR/"
echo "================================================================"
ls -lh "$OUTPUT_DIR/SmartOfficeAnnouncer.exe"
