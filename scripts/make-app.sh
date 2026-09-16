#!/bin/bash
# (Re)cria o "Estus Brain.app" em ~/Applications apontando para este repo.
# Rode de novo se mover o projeto de pasta ou mexer no ícone.
#
#   scripts/make-app.sh              # usa scripts/icon/EstusBrain.icns
#   scripts/make-app.sh --icon       # redesenha o .icns a partir do HTML antes
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$HOME/Applications/Estus Brain.app"
ICNS="$ROOT/scripts/icon/EstusBrain.icns"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

if [ "${1:-}" = "--icon" ]; then
  # Sem rasterizador de SVG na máquina; o Chrome headless faz o papel.
  [ -x "$CHROME" ] || { echo "Google Chrome não encontrado" >&2; exit 1; }
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --screenshot="$tmp/icon.png" --window-size=1024,1024 \
    --default-background-color=00000000 "$ROOT/scripts/icon/estus-icon.html" 2>/dev/null
  iset="$tmp/EstusBrain.iconset"; mkdir -p "$iset"
  gen() { sips -z "$1" "$1" "$tmp/icon.png" --out "$iset/$2" >/dev/null; }
  gen 16  icon_16x16.png;   gen 32  icon_16x16@2x.png
  gen 32  icon_32x32.png;   gen 64  icon_32x32@2x.png
  gen 128 icon_128x128.png; gen 256 icon_128x128@2x.png
  gen 256 icon_256x256.png; gen 512 icon_256x256@2x.png
  gen 512 icon_512x512.png; cp "$tmp/icon.png" "$iset/icon_512x512@2x.png"
  iconutil -c icns "$iset" -o "$ICNS"
  echo "ícone redesenhado: scripts/icon/EstusBrain.icns"
fi

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$ICNS" "$APP/Contents/Resources/EstusBrain.icns"

cat >"$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>              <string>Estus Brain</string>
  <key>CFBundleDisplayName</key>       <string>Estus Brain</string>
  <key>CFBundleIdentifier</key>        <string>com.rafael.estusbrain</string>
  <key>CFBundleVersion</key>           <string>1.0</string>
  <key>CFBundleShortVersionString</key><string>1.0</string>
  <key>CFBundlePackageType</key>       <string>APPL</string>
  <key>CFBundleExecutable</key>        <string>EstusBrain</string>
  <key>CFBundleIconFile</key>          <string>EstusBrain</string>
  <key>NSHighResolutionCapable</key>   <true/>
  <key>LSMinimumSystemVersion</key>    <string>12.0</string>
</dict>
</plist>
PLIST

# Fino de propósito: a lógica vive no repo, versionada.
cat >"$APP/Contents/MacOS/EstusBrain" <<SH
#!/bin/bash
exec "$ROOT/scripts/estus-brain.sh" "\$@"
SH
chmod +x "$APP/Contents/MacOS/EstusBrain"

touch "$APP"   # faz o Dock/Finder relerem o ícone
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$APP" 2>/dev/null || true
echo "criado: $APP"
echo "apontando para: $ROOT/scripts/estus-brain.sh"
