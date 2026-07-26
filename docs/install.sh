#!/bin/sh
# claude-status-line-go installer for macOS and Linux.
#
#   curl -fsSL https://okano.dev/claude-status-line-go/install.sh | sh
#
# Everything lives inside main(), which is only invoked on the very last line.
# A truncated download therefore defines some functions and does nothing, rather
# than executing half an install.
#
# Environment overrides:
#   VERSION      pin a version instead of taking the latest, e.g. VERSION=1.5.0
#   INSTALL_DIR  where the binary goes (default: /usr/local/bin if writable,
#                otherwise ~/.local/bin)
#
# This script never calls sudo. If /usr/local/bin isn't writable it installs to
# ~/.local/bin and tells you, rather than escalating privileges on your behalf.

set -eu

REPO='williamokano/claude-status-line-go'
BIN='claude-status-line-go'

say() { printf '%s\n' "$*"; }
err() { printf '%s: %s\n' "$BIN" "$*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

detect_os() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$os" in
	linux | darwin) printf '%s' "$os" ;;
	*) err "unsupported OS: $(uname -s) — Windows install steps are at https://okano.dev/claude-status-line-go/#install" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) printf 'amd64' ;;
	arm64 | aarch64) printf 'arm64' ;;
	*) err "unsupported architecture: $arch" ;;
	esac
}

# Follows the /releases/latest redirect instead of the REST API, which is rate
# limited to 60 requests/hour for anonymous callers.
latest_version() {
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
		err 'could not reach GitHub to resolve the latest version'
	v=${url##*/tag/v}
	case "$v" in
	'' | *[!0-9.]*) err "could not parse a version out of: $url" ;;
	esac
	printf '%s' "$v"
}

verify() { # <file> <checksums-file>, run from the directory holding both
	if command -v sha256sum >/dev/null 2>&1; then
		grep " $1\$" "$2" | sha256sum -c - >/dev/null
	elif command -v shasum >/dev/null 2>&1; then
		grep " $1\$" "$2" | shasum -a 256 -c - >/dev/null
	else
		err 'neither sha256sum nor shasum is available, so the download cannot be verified'
	fi
}

choose_dir() {
	if [ -n "${INSTALL_DIR:-}" ]; then
		printf '%s' "$INSTALL_DIR"
	elif [ -w /usr/local/bin ]; then
		printf '/usr/local/bin'
	else
		printf '%s' "$HOME/.local/bin"
	fi
}

main() {
	need curl
	need tar

	os=$(detect_os)
	arch=$(detect_arch)
	version=${VERSION:-$(latest_version)}
	version=${version#v}

	file="${BIN}_${version}_${os}_${arch}.tar.gz"
	base="https://github.com/$REPO/releases/download/v${version}"

	tmp=$(mktemp -d) || err 'could not create a temporary directory'
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "downloading $BIN v$version ($os/$arch)"
	curl -fsSL -o "$tmp/$file" "$base/$file" || err "download failed: $base/$file"
	curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || err 'could not download checksums.txt'

	(cd "$tmp" && verify "$file" checksums.txt) || err 'checksum verification failed — not installing'
	say 'checksum ok'

	tar -xzf "$tmp/$file" -C "$tmp" "$BIN" || err 'could not extract the binary from the archive'

	dir=$(choose_dir)
	mkdir -p "$dir" || err "could not create $dir"
	cp "$tmp/$BIN" "$dir/$BIN" || err "could not write to $dir — set INSTALL_DIR to somewhere you own"
	chmod 755 "$dir/$BIN"

	say "installed $dir/$BIN"

	case ":$PATH:" in
	*":$dir:"*) ;;
	*)
		say ''
		say "note: $dir is not on your PATH"
		;;
	esac

	say ''
	say "next:  $BIN install"
}

main "$@"
