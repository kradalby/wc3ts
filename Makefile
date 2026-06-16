.PHONY: lint test build all icons icons-verify

lint:
	golangci-lint run ./...

test:
	gotestsum --format testname ./...

build:
	go build -o wc3ts ./cmd/wc3ts

all: lint test build

# ── Icons / desktop integration ─────────────────────────────────────────────
# Regenerate every committed icon artifact from the source art. Maintainer-only;
# the build itself just consumes the committed outputs. Needs imagemagick,
# libicns (png2icns) and go on PATH (all in the nix devShell), e.g.:
#   nix develop --command make icons
GOVERSIONINFO := github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0
ICON_DIR      := assets/icons
ICON_SRC      := $(ICON_DIR)/icon-source.png
ICON_MASTER   := $(ICON_DIR)/icon-master.png
HICOLOR       := $(ICON_DIR)/hicolor
ICON_SIZES    := 16 22 24 32 48 64 128 256 512
ICNS_SIZES    := 16 32 48 128 256 512
ICO_SIZES     := 256,128,64,48,32,16

icons:
	# 1024x1024 master: pad the landscape source to a square black canvas
	# (seamless with the art's black field), then downscale.
	magick $(ICON_SRC) -background black -gravity center -extent 1536x1536 -resize 1024x1024 $(ICON_MASTER)
	# Windows multi-resolution .ico
	magick $(ICON_MASTER) -define icon:auto-resize=$(ICO_SIZES) $(ICON_DIR)/wc3ts.ico
	# Linux hicolor theme PNGs
	for s in $(ICON_SIZES); do \
		mkdir -p $(HICOLOR)/$${s}x$${s}/apps; \
		magick $(ICON_MASTER) -resize $${s}x$${s} $(HICOLOR)/$${s}x$${s}/apps/wc3ts.png; \
	done
	# macOS .icns (libicns needs individual square PNGs)
	tmp=$$(mktemp -d); \
	for s in $(ICNS_SIZES); do magick $(ICON_MASTER) -resize $${s}x$${s} $$tmp/icon_$${s}.png; done; \
	png2icns $(ICON_DIR)/wc3ts.icns $$tmp/icon_16.png $$tmp/icon_32.png $$tmp/icon_48.png \
		$$tmp/icon_128.png $$tmp/icon_256.png $$tmp/icon_512.png; \
	rm -rf $$tmp
	# Windows resource objects (icon + version info) auto-linked into the .exe.
	# -platform-specific writes resource_windows_<arch>.syso into the cwd.
	cd cmd/wc3ts && go run $(GOVERSIONINFO) \
		-icon ../../$(ICON_DIR)/wc3ts.ico \
		-platform-specific \
		../../packaging/windows/versioninfo.json
	# We only ship amd64/arm64; drop the unused 386/arm objects.
	rm -f cmd/wc3ts/resource_windows_386.syso cmd/wc3ts/resource_windows_arm.syso

icons-verify:
	magick identify $(ICON_DIR)/wc3ts.ico
	test -f $(ICON_DIR)/wc3ts.icns
	test -f cmd/wc3ts/resource_windows_amd64.syso
	test -f cmd/wc3ts/resource_windows_arm64.syso
