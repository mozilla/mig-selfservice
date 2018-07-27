// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor: Julien Vehent jvehent@mozilla.com [:ulfr]
package main

import (
	_ "github.com/mozilla/mig/modules/agentdestroy"
	//_ "github.com/mozilla/mig/modules/examplepersist"
	_ "github.com/mozilla/mig/modules/file"
	//_ "github.com/mozilla/mig/modules/fswatch"
	_ "github.com/mozilla/mig/modules/memory"
	_ "github.com/mozilla/mig/modules/netstat"
	_ "github.com/mozilla/mig/modules/ping"
	_ "github.com/mozilla/mig/modules/pkg"
	_ "github.com/mozilla/mig/modules/scribe"
	_ "github.com/mozilla/mig/modules/timedrift"
	//_ "github.com/mozilla/mig/modules/yara"
	//_ "github.com/mozilla/mig/modules/example"
)
