#!/bin/sh

CHECKSUMS_FILE="build/checksums.txt"
BINARY_NAME="opengist"
TARGETS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 linux/armv6 linux/armv7 linux/386 windows/amd64"
VERSION=$(git describe --tags | sed 's/^v//')
VERSION_PKG="github.com/thomiceli/opengist/internal/config.OpengistVersion"

if [ -z "$VERSION" ]; then
    echo "Error: Could not retrieve version from git tags. Exiting..."
    exit 1
fi

for TARGET in $TARGETS; do
    GOOS=${TARGET%/*}
    GOARCH=${TARGET#*/}

    case $GOOS-$GOARCH in
        linux-armv6)
            GOARCH="arm"
            GOARM=6
            ;;
        linux-armv7)
            GOARCH="arm"
            GOARM=7
            ;;
        *)
            unset GOARM
            ;;
    esac

    OUTPUT_PARENT_DIR="build/$GOOS-$GOARCH${GOARM:+v$GOARM}-$VERSION"
    OUTPUT_DIR="$OUTPUT_PARENT_DIR/$BINARY_NAME"
    OUTPUT_FILE="$OUTPUT_DIR/$BINARY_NAME"

    if [ "$GOOS" = "windows" ]; then
        OUTPUT_FILE="$OUTPUT_FILE.exe"
    fi

    echo "Building version $VERSION for $GOOS/$GOARCH${GOARM:+v$GOARM}..."
    mkdir -p $OUTPUT_DIR
    env GOOS=$GOOS GOARCH=$GOARCH GOARM=$GOARM CGO_ENABLED=0 go build -tags fs_embed -ldflags "-X $VERSION_PKG=v$VERSION" -o $OUTPUT_FILE
    cp README.md $OUTPUT_DIR
    cp LICENSE $OUTPUT_DIR
    cp config.yml $OUTPUT_DIR

    if [ $? -ne 0 ]; then
        echo "Error building for $GOOS/$GOARCH${GOARM:+v$GOARM}. Exiting..."
        exit 1
    fi

    # Archive the binary with README and LICENSE
    echo "Archiving for $GOOS/$GOARCH${GOARM:+v$GOARM}..."
    if [ "$GOOS" = "windows" ]; then
        # ZIP for Windows
        cd $OUTPUT_PARENT_DIR && zip -r "../$BINARY_NAME$VERSION-$GOOS-$GOARCH${GOARM:+v$GOARM}.zip" "$BINARY_NAME/" && cd - > /dev/null
        sha256sum "build/$BINARY_NAME$VERSION-$GOOS-$GOARCH${GOARM:+v$GOARM}.zip" | awk '{print $1 " " substr($2,7)}' >> $CHECKSUMS_FILE
    else
        # tar.gz for other platforms
        tar -czf "build/$BINARY_NAME$VERSION-$GOOS-$GOARCH${GOARM:+v$GOARM}.tar.gz" -C $OUTPUT_PARENT_DIR "$BINARY_NAME"
        sha256sum "build/$BINARY_NAME$VERSION-$GOOS-$GOARCH${GOARM:+v$GOARM}.tar.gz" | awk '{print $1 " " substr($2,7)}' >> $CHECKSUMS_FILE
    fi
done

echo "Build and archiving complete."
