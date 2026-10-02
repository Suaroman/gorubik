#!/usr/bin/env bash
# Build gorubik.
#
#   ./build.sh              host binary            -> bin/gorubik
#   ./build.sh windows      cross from WSL to PE32+ -> bin/gorubik.exe
#   ./build.sh windows release
#                           same, stripped and with the GUI subsystem so it
#                           launches without a console window
#
# The Windows cross-build needs a mingw-w64 C compiler because GLFW is cgo;
# plain GOOS=windows without CGO_ENABLED=1 fails to link, not to compile.
set -euo pipefail
cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  if [ -x /usr/local/go/bin/go ]; then export PATH=/usr/local/go/bin:$PATH; fi
fi

target="${1:-host}"
variant="${2:-debug}"

case "$target" in
  host)
    echo "building bin/gorubik for $(go env GOOS)/$(go env GOARCH)"
    go build -o bin/gorubik ./cmd/gorubik
    echo "run: ./bin/gorubik"
    ;;
  windows)
    cc=x86_64-w64-mingw32-gcc
    if ! command -v "$cc" >/dev/null 2>&1; then
      echo "error: $cc not found. On WSL/Debian: sudo apt-get install -y mingw-w64" >&2
      exit 1
    fi
    # The default build keeps symbols and the console so the startup banner,
    # -bench and -pass-times are readable. Release strips and detaches the
    # console, which means those prints go nowhere.
    ldflags=""
    if [ "$variant" = "release" ]; then
      ldflags="-s -w -H=windowsgui"
    fi
    echo "building bin/gorubik.exe for windows/amd64 ($variant)"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="$cc" \
      go build -ldflags "$ldflags" -o bin/gorubik.exe ./cmd/gorubik
    ls -l bin/gorubik.exe
    # Inside WSL the same directory is reachable from Windows as a UNC path, which
    # is what lets one build serve both systems. cmd.exe swallows single
    # backslashes in some quoting contexts, hence the doubled ones.
    if [ -n "${WSL_DISTRO_NAME:-}" ]; then
      winpath="${PWD//\//\\}"
      unc="\\\\wsl.localhost\\${WSL_DISTRO_NAME}${winpath}\\bin\\gorubik.exe"
      echo "run from WSL:  cmd.exe /c '$unc'"
      echo "               (or double-click bin\\\\gorubik.exe from Explorer)"
    else
      echo "run:  bin/gorubik.exe"
    fi
    ;;
  test)
    gofmt -l . && go vet ./... && go test ./... && go run ./cmd/gorubik -verify
    ;;
  *)
    echo "usage: $0 [host|windows [debug|release]|test]" >&2
    exit 2
    ;;
esac
