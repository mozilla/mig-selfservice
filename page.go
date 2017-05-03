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
</head>
<body>
<div>
  <p>Welcome, {{.RemoteUser}}.</p>
</div>
<div>
  <table>
    <thead>
      <tr>
      <td>Device slot</td><td>Assigned key<td><td>Action</td><td>In use?</td>
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
