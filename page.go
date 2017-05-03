// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor: Aaron Meihm ameihm@mozilla.com [:alm]
package main

import (
	"bufio"
	"bytes"
	"html/template"
)

var mainTmpl = `<html>
<head>
<script src="static/jquery-3.2.1.min.js" type="text/javascript"></script>
<script src="static/selfservice.js" type="text/javascript"></script>
<link rel="stylesheet" type="text/css" href="static/selfservice.css">
</head>
<body>
<div>
<img src="static/mig-logo-transparent.png" width="25%">
</div>
<div>
<h1>MIG self-service portal</h1>
</div>
<div>
  <p>Welcome, <i>{{.RemoteUser}}.</i></p>
  <p>This is the self-service portal for <a href="http://mig.mozilla.org">Mozilla
  Investigator</a>. Here you can download MIG for your workstation devices, and create
  your own keys to allow you to install the agent. You can create up to 3 keys to use
  on end-point devices that support MIG.</p>
  <p>Mozilla Infosec uses the MIG agent to rapidly respond to incidents and help
  identify security issues that may have occurred within the organization.</p>
  <p>After generating a key in a key slot, be sure to note the key as it will only be
  displayed upon initial creation.</p>
</div>
<div>
  <table>
    <thead>
      <tr>
      <td>Device slot</td><td>Assigned key</td><td>Action</td><td>Key last used</td>
      </tr>
    </thead>
    <tbody>
      <tr id="slot1"><td>1</td><td>Loading</td><td>Loading</td><td>Loading</td></tr>
      <tr id="slot2"><td>2</td><td>Loading</td><td>Loading</td><td>Loading</td></tr>
      <tr id="slot3"><td>3</td><td>Loading</td><td>Loading</td><td>Loading</td></tr>
    </tbody>
  </table>
</div>
</body>
</html>
`

type templateData struct {
	RemoteUser string
}

func (t *templateData) importFromRequest(r requestDetails) {
	t.RemoteUser = r.remoteUser
}

func renderMainPage(rdetails requestDetails) (string, error) {
	var outbuf bytes.Buffer

	tdata := templateData{}
	tdata.importFromRequest(rdetails)
	t, err := template.New("main").Parse(mainTmpl)
	if err != nil {
		return "", err
	}
	bw := bufio.NewWriter(&outbuf)
	err = t.Execute(bw, tdata)
	if err != nil {
		return "", err
	}
	bw.Flush()
	return outbuf.String(), nil
}
