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

# signing_key is the public key release packages must be signed with.  It must
# match defaultSigningKey in internal/updater/signature.go.
signing_key='-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAaadffatSWma6HEJPN3uVcpKDIxFhnltcZGaQM2jWmSM=
-----END PUBLIC KEY-----'
readonly signing_key

log "installing AdGuard Escolar (${os}/amd64) into ${install_dir}"

tmp_dir="$(mktemp -d)"
readonly tmp_dir
trap 'rm -rf "$tmp_dir"' EXIT

# download URL FILE downloads URL into FILE.
download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fL -o "$2" "$1"
	elif command -v fetch >/dev/null 2>&1; then
		fetch -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -O "$2" "$1"
	else
		error_exit 'curl, fetch, or wget is required to run this script'
	fi
}

# verify_package FILE checks FILE against its detached signature FILE.sig.  The
# signed message is the same one the built-in updater checks, see
# SignedMessage in internal/updater/signature.go.
verify_package() {
	if ! command -v openssl >/dev/null 2>&1; then
		error_exit 'openssl is required to verify the package signature'
	fi

	printf '%s\n' "$signing_key" > "${tmp_dir}/signing_key.pem"
	openssl base64 -d -A -in "${1}.sig" -out "${1}.sig.bin"

	sum="$(openssl dgst -sha256 -r "$1" | cut -d ' ' -f 1)"
	printf 'escolarapp-shield-update-v1\n%s\n' "$sum" > "${1}.msg"

	if ! openssl pkeyutl -verify -pubin -inkey "${tmp_dir}/signing_key.pem" \
		-rawin -in "${1}.msg" -sigfile "${1}.sig.bin" >/dev/null 2>&1
	then
		error_exit 'the package signature is not valid; refusing to install it'
	fi
}

log "downloading ${url}"
download "$url" "${tmp_dir}/${pkg_name}"
download "${url}.sig" "${tmp_dir}/${pkg_name}.sig"

log 'verifying the package signature'
verify_package "${tmp_dir}/${pkg_name}"

log 'unpacking'
tar -C "$tmp_dir" -x -z -f "${tmp_dir}/${pkg_name}"

was_installed=0
if [ -x "${install_dir}/AdGuardHome" ]; then
	was_installed=1
	log "existing installation found at ${install_dir}, stopping it before replacing the binary"
	"${install_dir}/AdGuardHome" -s stop 2>/dev/null || true
fi
readonly was_installed

mkdir -p "$install_dir"
install -m 0755 "${tmp_dir}/AdGuardHome/AdGuardHome" "${install_dir}/AdGuardHome"

cd "$install_dir"

if [ "$was_installed" -eq 1 ]; then
	# Re-register the service from scratch with the new binary, rather than
	# using its own -s restart: restarting immediately after the manual stop
	# above raced with the OS releasing the old process's port/pidfile and
	# failed outright on FreeBSD. Uninstall-then-install sidesteps that.
	log "re-registering the system service with the new binary"
	./AdGuardHome -s uninstall 2>/dev/null || true
	sleep 1
fi

log "registering and starting the system service"
./AdGuardHome -s install

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
