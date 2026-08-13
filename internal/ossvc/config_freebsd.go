//go:build freebsd

package ossvc

import (
	"github.com/kardianos/service"
)

// configureServiceOptions defines additional settings of the service
// configuration on FreeBSD.  conf must not be nil.
func configureOSOptions(conf *service.Config) {
	conf.Option["SysvScript"] = freeBSDScript
}

// freeBSDScript is the source of the daemon script for FreeBSD/OPNsense.
const freeBSDScript = `#!/bin/sh
# PROVIDE: {{.Name}}
# REQUIRE: SERVERS NETWORKING
# BEFORE: DAEMON
# KEYWORD: shutdown

. /etc/rc.subr

name="{{.Name}}"
{{.Name}}_enable=${{{.Name}}_enable:-"YES"}
{{.Name}}_env="IS_DAEMON=1"
{{.Name}}_user="root"
pidfile_child="/var/run/${name}.pid"
pidfile="/var/run/${name}_daemon.pid"
command="/usr/sbin/daemon"
daemon_args="-P ${pidfile} -p ${pidfile_child} -r -t ${name}"
command_args="${daemon_args} {{.Path}}{{range .Arguments}} {{.}}{{end}}"

load_rc_config $name
run_rc_command "$1"
`
