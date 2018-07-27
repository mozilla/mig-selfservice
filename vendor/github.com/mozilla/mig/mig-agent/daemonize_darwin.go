// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor:
// - Julien Vehent jvehent@mozilla.com [:ulfr]
package main

import (
	"fmt"
	"github.com/mozilla/mig"
	"os"
	"os/exec"
)

// On MacOS, launchd takes care of keeping processes alive. The daemonization
// procedure consist of installing and starting the service, then exiting.
// Launchd will take care of daemonizing the agent
func daemonize(orig_ctx Context, upgrading bool) (ctx Context, err error) {
	ctx = orig_ctx
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("daemonize() -> %v", e)
		}
		ctx.Channels.Log <- mig.Log{Desc: "leaving daemonize()"}.Debug()
	}()

	if os.Getppid() == 1 {
		ctx.Channels.Log <- mig.Log{Desc: "Parent process is PID 1"}.Debug()
		// if controlled by launchd, we tell the agent
		// to not respawn itself. launchd will do it
		ctx.Channels.Log <- mig.Log{Desc: "Running as a service."}.Debug()
		ctx.Agent.Respawn = false
	} else {
		// install the service, start it, and exit
		if MUSTINSTALLSERVICE {
			ctx, err = serviceDeploy(ctx)
			if err != nil {
				panic(err)
			}
			ctx.Channels.Log <- mig.Log{Desc: "Service deployed. Exit."}.Debug()
		} else {
			// we are not in foreground mode, and we don't want a service installation
			// so just fork in foreground mode, and exit the current process
			cmd := exec.Command(ctx.Agent.BinPath, "-f")
			err = cmd.Start()
			if err != nil {
				ctx.Channels.Log <- mig.Log{Desc: fmt.Sprintf("Failed to spawn new agent from '%s': '%v'", ctx.Agent.BinPath, err)}.Err()
				return ctx, err
			}
			ctx.Channels.Log <- mig.Log{Desc: "Started new foreground agent. Exit."}.Debug()
		}
		os.Exit(0)
	}
	return
}

func installCron(ctx Context) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("installCron() -> %v", e)
		}
		ctx.Channels.Log <- mig.Log{Desc: "leaving installCron()"}.Debug()
	}()
	panic("mig-agent doesn't have a cronjob for darwin.")
	return
}
