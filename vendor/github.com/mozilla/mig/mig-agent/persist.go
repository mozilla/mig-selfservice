// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor:
// - Aaron Meihm ameihm@mozilla.com [:alm]
package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mozilla/mig"
	"github.com/mozilla/mig/modules"

	"gopkg.in/gcfg.v1"
)

// persistModuleRegister maintains a map of the running persistent modules, and
// any socket specification registered for that module.
//
// Socket specifications tell the mig-agent how it should query a running
// persistent module. The specification indicates where a running persistent
// module has registered as listening.
//
// Socket specifications are format "family:address". For example, for a UNIX
// domain socket, you might have "unix:/var/lib/mig/mymodule.sock" registered
// for mymodule.
//
// For platforms that do not support domain sockets, the network can be used in
// which case you might have something like "tcp:127.0.0.1:55000".
type persistModuleRegister struct {
	modules map[string]*string
	sync.Mutex
}

// Get a socket specification registered for a given persistent module
func (p *persistModuleRegister) get(modname string) (string, error) {
	p.Lock()
	defer p.Unlock()
	sv, ok := p.modules[modname]
	if !ok || sv == nil {
		return "", fmt.Errorf("module %v is not registered", modname)
	}
	return *sv, nil
}

// Register a socket specification for persistent module modname
func (p *persistModuleRegister) register(modname string, spec string) {
	p.Lock()
	defer p.Unlock()
	p.modules[modname] = &spec
}

// Remove a socket specification for a persistent module
func (p *persistModuleRegister) remove(modname string) {
	p.Lock()
	defer p.Unlock()
	p.modules[modname] = nil
}

var persistModRegister persistModuleRegister

// Load the configuration file for a persistent module if it exists, and return it
// as a JSON byte slice so we can send it from the agent to the module after the
// module is started. If the configuration file cannot be loaded, just return the
// config struct for the module uninitialized.
func getPersistConfig(modname string) (ret interface{}) {
	cfg := modules.Available[modname].NewRun().(modules.PersistRunner).PersistModConfig()
	confpath := path.Join(MODULECONFIGDIR, modname+".cfg")
	// An error here isn't fatal, we just continue with cfg as is
	gcfg.ReadFileInto(cfg, confpath)
	ret = cfg
	return
}

// Start all the persistent modules available to the agent.
func startPersist(ctx *Context) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("startPersist() -> %v", e)
		}
	}()
	ctx.Channels.Log <- mig.Log{Desc: "initializing any persistent modules"}.Debug()

	persistModRegister.modules = make(map[string]*string)

	for k, v := range modules.Available {
		if _, ok := v.NewRun().(modules.PersistRunner); ok {
			err = startPersistModule(ctx, k)
			if err != nil {
				panic(err)
			}
		}
	}
	return
}

// Starts a given persistent module.
func startPersistModule(ctx *Context, name string) (err error) {
	ctx.Channels.Log <- mig.Log{Desc: fmt.Sprintf("starting persistent module %v", name)}.Info()
	go managePersistModule(ctx, name)
	return
}

// Persistent module management function used in the agent. For each persistent module
// the agent is running, this function will execute in a go-routine.
func managePersistModule(ctx *Context, name string) {
	var (
		cmd        *exec.Cmd
		isRunning  bool
		pipeout    modules.ModuleWriter
		pipein     modules.ModuleReader
		err        error
		failDelay  bool
		killModule bool
		inChan     chan modules.Message
		lastPing   time.Time
	)

	logfunc := func(f string, a ...interface{}) {
		buf := fmt.Sprintf(f, a...)
		buf = fmt.Sprintf("[%v] %v", name, buf)
		ctx.Channels.Log <- mig.Log{Desc: buf}.Info()
	}

	pingtick := time.Tick(time.Second * 10)

	for {
		if failDelay {
			time.Sleep(time.Second * 10)
			failDelay = false
		}

		if !isRunning {
			logfunc("starting module")
			lastPing = time.Now()
			cmd = exec.Command(ctx.Agent.BinPath, "-P", strings.ToLower(name))
			cmdpipeout, err := cmd.StdinPipe()
			if err != nil {
				logfunc("error creating stdin pipe, %v", err)
				failDelay = true
				continue
			}
			pipeout = modules.NewModuleWriter(cmdpipeout)
			cmdpipein, err := cmd.StdoutPipe()
			if err != nil {
				logfunc("error creating stdout pipe, %v", err)
				failDelay = true
				continue
			}
			pipein = modules.NewModuleReader(cmdpipein)
			cfg := getPersistConfig(name)
			err = cmd.Start()
			if err != nil {
				logfunc("error starting module, %v", err)
				failDelay = true
				continue
			}
			inChan = make(chan modules.Message, 0)

			go func() {
				for {
					msg, err := modules.ReadInput(pipein)
					if err != nil {
						logfunc("%v", err)
						close(inChan)
						break
					}
					inChan <- msg
				}
			}()

			isRunning = true

			// The module is now running, send any configuration parameters we have
			// to it.
			cm, err := modules.MakeMessageConfig(cfg)
			if err != nil {
				// This should never happen, but if it does we will just
				// kill the executing module as we are unable to send any
				// configuration to it
				killModule = true
				break
			}
			err = modules.WriteOutput(cm, pipeout)
			if err != nil {
				// XXX This should be revisited, both here and later on when
				// sending a ping. If this write fails, we just assume the
				// process is down, where it may not be.
				logfunc("config write failed, %v", err)
				isRunning = false
				persistModRegister.remove(name)
				failDelay = true
				continue
			}
		}
		select {
		case msg, ok := <-inChan:
			if !ok {
				err = cmd.Wait()
				logfunc("module is down, %v", err)
				isRunning = false
				persistModRegister.remove(name)
				failDelay = true
				break
			}
			switch msg.Class {
			case modules.MsgClassPing:
				lastPing = time.Now()
			case modules.MsgClassLog:
				var lp modules.LogParams
				buf, err := json.Marshal(msg.Parameters)
				if err != nil {
					logfunc("%v", err)
					break
				}
				err = json.Unmarshal(buf, &lp)
				if err != nil {
					logfunc("%v", err)
					break
				}
				logfunc("(module log) %v", lp.Message)
			case modules.MsgClassRegister:
				var rp modules.RegParams
				buf, err := json.Marshal(msg.Parameters)
				if err != nil {
					logfunc("%v", err)
					break
				}
				err = json.Unmarshal(buf, &rp)
				if err != nil {
					logfunc("%v", err)
					break
				}
				persistModRegister.register(name, rp.SockPath)
				logfunc("module has registered at %v", rp.SockPath)
			default:
				logfunc("unknown message class")
				killModule = true
				break
			}
		case _ = <-pingtick:
			// If we haven't received a reply in the past 3 cycles we will
			// kill the module
			if time.Now().Sub(lastPing) >= time.Duration(30*time.Second) {
				logfunc("no ping response from module, killing")
				killModule = true
				break
			}

			pm, err := modules.MakeMessage("ping", nil, false)
			if err != nil {
				// Failure here should not occur but does not
				// mean the module is down
				logfunc("failed to create ping, %v", err)
				break
			}
			err = modules.WriteOutput(pm, pipeout)
			if err != nil {
				logfunc("ping failed, %v", err)
				isRunning = false
				persistModRegister.remove(name)
				failDelay = true
				break
			}
		}

		if killModule {
			logfunc("killing module")
			err = cmd.Process.Kill()
			if err != nil {
				logfunc("failed to kill module, %v", err)
				// If this happens we are in a bad state, return from here
				// as we cannot recover
				return
			}
			_ = cmd.Wait()
			isRunning = false
			persistModRegister.remove(name)
			failDelay = true
			killModule = false
		}
	}
}
