#!/bin/sh

# AdGuard Escolar installation script.
#
# Downloads the latest build from this fork (not the official AdGuard Home
# release) and installs it as a system service. Intended for FreeBSD/OPNsense
# and Linux amd64 hosts.
#
# Usage: run as root (or with sudo):
#   curl -fsSL https://github.com/JosuhaSanhueza/adguard_escolar/raw/master/scripts/install-escolar.sh | sh
# or, if you already have the repo checked out:
#   sudo sh scripts/install-escolar.sh

set -e -f -u

repo='JosuhaSanhueza/adguard_escolar'
readonly repo

log() {
	echo "$1" 1>&2
}

error_exit() {
	log "error: $1"

	exit 1
}

if [ "$(id -u)" -ne 0 ]; then
	error_exit 'this script must be run as root (use sudo)'
fi

os="$(uname -s)"
case "$os" in
'FreeBSD')
	os='freebsd'
	install_dir='/usr/local/bin'
	;;
'Linux')
	os='linux'
	install_dir='/opt/AdGuardHome'
	;;
*)
	error_exit "unsupported operating system: '$os' (this fork only builds linux_amd64 and freebsd_amd64)"
	;;
esac
readonly os install_dir

arch="$(uname -m)"
case "$arch" in
'x86_64' | 'amd64')
	# All right, go on.
	;;
*)
	error_exit "unsupported cpu architecture: '$arch' (this fork only builds amd64)"
	;;
esac

pkg_name="AdGuardHome_${os}_amd64.tar.gz"
url="https://github.com/${repo}/raw/master/${pkg_name}"
readonly pkg_name url

log "installing AdGuard Escolar (${os}/amd64) into ${install_dir}"

tmp_dir="$(mktemp -d)"
readonly tmp_dir
trap 'rm -rf "$tmp_dir"' EXIT

log "downloading ${url}"
if command -v curl >/dev/null 2>&1; then
	curl -fL -o "${tmp_dir}/${pkg_name}" "$url"
elif command -v fetch >/dev/null 2>&1; then
	fetch -o "${tmp_dir}/${pkg_name}" "$url"
elif command -v wget >/dev/null 2>&1; then
	wget -O "${tmp_dir}/${pkg_name}" "$url"
else
	error_exit 'curl, fetch, or wget is required to run this script'
fi

log 'unpacking'
tar -C "$tmp_dir" -x -z -f "${tmp_dir}/${pkg_name}"

if [ -x "${install_dir}/AdGuardHome" ]; then
	log "existing installation found at ${install_dir}, stopping it before replacing the binary"
	"${install_dir}/AdGuardHome" -s stop 2>/dev/null || true
fi

mkdir -p "$install_dir"
install -m 0755 "${tmp_dir}/AdGuardHome/AdGuardHome" "${install_dir}/AdGuardHome"

log "registering and starting the system service"
cd "$install_dir"
if [ -x "${install_dir}/AdGuardHome" ] && "${install_dir}/AdGuardHome" -s status >/dev/null 2>&1; then
	# Already installed as a service (e.g. reinstall/update): just restart it
	# to pick up the new binary.
	./AdGuardHome -s restart
else
	./AdGuardHome -s install
fi

cat <<EOF

AdGuard Escolar is installed and running from ${install_dir}.

Finish setup by opening the web UI shown above and completing the
installation wizard (bind address, admin user, and password).

Once the wizard is done, if you already have a profile exported from
another installation (see --export-profile/--import-profile), you can
import it and restart the service to apply it:

  ${install_dir}/AdGuardHome --import-profile /path/to/perfil.yaml
  ${install_dir}/AdGuardHome -s restart
EOF
