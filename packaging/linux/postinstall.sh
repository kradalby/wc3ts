#!/bin/sh
# Refresh the desktop and icon caches so the wc3ts launcher entry and icon show
# up immediately after install. Best-effort: tools may be absent on minimal
# systems, which is fine.
set -e

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi

exit 0
