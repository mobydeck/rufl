# Default recipe to show available commands
default:
    @just --list

# Build flags for optimized binaries
build_flags := "-ldflags='-s -w' -trimpath"

# Build for the current platform
build:
    go build {{build_flags}} -o rufl

# Clean build artifacts
clean:
    rm -rf dist
    rm -f rufl rufl.exe

# Create distribution directory
create-dist:
    mkdir -p dist

# Build for a specific platform and architecture
build-for os arch:
    @echo "Building for {{os}} ({{arch}})..."
    @mkdir -p dist
    @if [ "{{os}}" = "windows" ]; then \
        GOOS={{os}} GOARCH={{arch}} go build {{build_flags}} -o dist/rufl-{{os}}-{{arch}}.exe; \
    else \
        GOOS={{os}} GOARCH={{arch}} go build {{build_flags}} -o dist/rufl-{{os}}-{{arch}}; \
    fi
    @if [ "{{os}}" = "windows" ]; then \
        echo "✓ Built dist/rufl-{{os}}-{{arch}}.exe"; \
    else \
        echo "✓ Built dist/rufl-{{os}}-{{arch}}"; \
    fi

# Build for all platforms (linux, macos, windows) and architectures (amd64, arm64)
build-all: clean create-dist
    just build-for linux amd64
    just build-for linux arm64
    just build-for darwin amd64
    just build-for darwin arm64
    just build-for windows amd64
    just build-for windows arm64
    @echo "All builds completed successfully!"
    @ls -la dist/

# Build for Linux amd64
build-linux-amd64:
    @just build-for linux amd64

# Build for Linux arm64
build-linux-arm64:
    @just build-for linux arm64

# Build for macOS amd64
build-macos-amd64:
    @just build-for darwin amd64

# Build for macOS arm64
build-macos-arm64:
    @just build-for darwin arm64

# Build for Windows amd64
build-windows-amd64:
    @just build-for windows amd64

# Build for Windows arm64
build-windows-arm64:
    @just build-for windows arm64

# Create archive for a specific build
create-archive os arch:
    @echo "Creating archive for {{os}}/{{arch}} with documentation..."
    @cp README.md dist/
    @if [ -f LICENSE ]; then cp LICENSE dist/; fi
    @if [ "{{os}}" = "windows" ]; then \
        cd dist && zip rufl-{{os}}-{{arch}}.zip rufl-{{os}}-{{arch}}.exe README.md $([ -f LICENSE ] && echo "LICENSE"); \
    else \
        cd dist && tar -czf rufl-{{os}}-{{arch}}.tar.gz rufl-{{os}}-{{arch}} README.md $([ -f LICENSE ] && echo "LICENSE"); \
    fi
    @rm -f dist/README.md dist/LICENSE 2>/dev/null
    @echo "✓ Archive created for {{os}}/{{arch}}"

# Create release archives
package: build-all
    @echo "Creating release archives..."
    just create-archive linux amd64
    just create-archive linux arm64
    just create-archive darwin amd64
    just create-archive darwin arm64
    just create-archive windows amd64
    just create-archive windows arm64
    @echo "✓ Created release archives in dist/"

# Run tests
test:
    go test -v ./...

# Install locally
install: build
    go install
