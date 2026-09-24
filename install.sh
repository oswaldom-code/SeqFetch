#!/bin/sh
# Installs the latest seqfetch release binary for the current OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/oswaldom-code/SeqFetch/main/install.sh | sh
#
# Environment overrides:
#   SEQFETCH_VERSION      tag to install, e.g. v1.2.0 (default: latest release)
#   SEQFETCH_INSTALL_DIR  target directory (default: /usr/local/bin, or ~/.local/bin if not writable)
#   SEQFETCH_BASE_URL     where to fetch archives from (default: the GitHub release assets)
set -eu

REPO="oswaldom-code/SeqFetch"
BINARY="seqfetch"

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }

detect_os() {
	case "$(uname -s)" in
		Linux)  echo linux ;;
		Darwin) echo darwin ;;
		*)      die "unsupported OS $(uname -s); download the Windows zip from https://github.com/$REPO/releases" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64)  echo amd64 ;;
		arm64|aarch64) echo arm64 ;;
		*)             die "unsupported architecture $(uname -m)" ;;
	esac
}

latest_tag() {
	curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
		| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1
}

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

pick_install_dir() {
	if [ -n "${SEQFETCH_INSTALL_DIR:-}" ]; then
		echo "$SEQFETCH_INSTALL_DIR"
	elif [ -w /usr/local/bin ]; then
		echo /usr/local/bin
	else
		echo "$HOME/.local/bin"
	fi
}

main() {
	need curl
	need tar

	os=$(detect_os)
	arch=$(detect_arch)

	tag="${SEQFETCH_VERSION:-}"
	if [ -z "$tag" ]; then
		tag=$(latest_tag)
		[ -n "$tag" ] || die "could not determine the latest release of $REPO"
	fi
	version="${tag#v}"

	base_url="${SEQFETCH_BASE_URL:-https://github.com/$REPO/releases/download/$tag}"
	archive="${BINARY}_${version}_${os}_${arch}.tar.gz"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT

	log "Downloading $archive ($tag)"
	curl -fsSL -o "$tmp/$archive" "$base_url/$archive"
	curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt"

	expected=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
	[ -n "$expected" ] || die "$archive not listed in checksums.txt"
	actual=$(sha256_of "$tmp/$archive")
	[ "$expected" = "$actual" ] || die "checksum mismatch for $archive"

	tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY"

	dir=$(pick_install_dir)
	mkdir -p "$dir"
	install -m 0755 "$tmp/$BINARY" "$dir/$BINARY"
	log "Installed $dir/$BINARY ($("$dir/$BINARY" --version))"

	case ":$PATH:" in
		*":$dir:"*) ;;
		*) log "note: $dir is not on your PATH; add it with: export PATH=\"$dir:\$PATH\"" ;;
	esac
}

main "$@"
