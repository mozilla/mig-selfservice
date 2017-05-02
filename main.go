// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor: Aaron Meihm ameihm@mozilla.com [:alm]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"mig.ninja/mig"
	"mig.ninja/mig/client"
	migdbsearch "mig.ninja/mig/database/search"
)

type config struct {
	APIUrl         string
	APIKey         string
	SkipVerifyCert bool
}

var cfg config

type remoteUserType int

const remoteUser remoteUserType = 0

type loadersReply struct {
	Loaders []mig.LoaderEntry `json:"loaders"`
}

type requestDetails struct {
	remoteUser string
	loaders    []mig.LoaderEntry
}

func (r *requestDetails) searchUserString() string {
	return "migss-" + r.remoteUser + "-%"
}

func (r *requestDetails) addKeys(cli client.Client) error {
	p := migdbsearch.NewParameters()
	p.Type = "loader"
	p.LoaderName = r.searchUserString()
	resources, err := cli.GetAPIResource("search?" + p.String())
	if err != nil {
		// Determine if it was a 404, if so this isn't an error and just return
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil
		}
		return err
	}
	for _, x := range resources.Collection.Items {
		for _, y := range x.Data {
			if y.Name != "loader" {
				continue
			}
			le, err := client.ValueToLoaderEntry(y.Value)
			if err != nil {
				return err
			}
			r.loaders = append(r.loaders, le)
		}
	}
	return r.validate()
}

func (r *requestDetails) validate() error {
	match, err := regexp.MatchString("^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\\.[a-zA-Z0-9-.]+$", r.remoteUser)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("invalid remoteUser")
	}
	return nil
}

// Convert data stored in the request context into a new requestDetails structure
// which will be passed around for the lifetime of the request
func newRequestDetails(req *http.Request) (ret requestDetails, err error) {
	var ok bool
	ti := context.Get(req, remoteUser)
	if ti == nil {
		return ret, fmt.Errorf("invalid remoteUser")
	}
	ret.remoteUser, ok = ti.(string)
	if !ok {
		return ret, fmt.Errorf("invalid remoteUser")
	}
	return ret, ret.validate()
}

func handleMain(rw http.ResponseWriter, req *http.Request) {
	rdetails, err := newRequestDetails(req)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	mp, err := renderMainPage(rdetails)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	fmt.Fprint(rw, mp)
}

func handleKeyStatus(rw http.ResponseWriter, req *http.Request) {
	rdetails, err := newRequestDetails(req)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	cli, err := newMIGClient()
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	err = rdetails.addKeys(cli)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	resp := loadersReply{}
	resp.Loaders = rdetails.loaders
	if resp.Loaders == nil {
		resp.Loaders = make([]mig.LoaderEntry, 0)
	}
	buf, err := json.Marshal(&resp)
	if err != nil {
		http.Error(rw, err.Error(), 500)
	}
	rw.Header().Set("Content-Type", "application/json")
	fmt.Fprint(rw, string(buf))
}

func handlePing(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprint(rw, "pong\n")
}

func newMIGClient() (ret client.Client, err error) {
	var cconf client.Configuration
	cconf.API.URL = cfg.APIUrl
	cconf.API.SkipVerifyCert = cfg.SkipVerifyCert
	cconf.GPG.UseAPIKeyAuth = cfg.APIKey

	ret, err = client.NewClient(cconf, "mig-selfservice")
	if err != nil {
		return
	}

	return
}

func setContext(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		context.Set(r, remoteUser, "ameihm@mozilla.com")
		h(w, r)
	}
}

func main() {
	var (
		err      error
		confpath string
	)

	flag.StringVar(&confpath, "confpath", "./mig-selfservice.yml", "path to configuration file")
	flag.Parse()
	cfgbuf, err := ioutil.ReadFile(confpath)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(cfgbuf, &cfg)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.HandleFunc("/ping", handlePing).Methods("GET")
	r.HandleFunc("/", setContext(handleMain)).Methods("GET")
	r.HandleFunc("/keystatus", setContext(handleKeyStatus)).Methods("GET")

	sp := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.PathPrefix("/static").Handler(sp)

	http.Handle("/", context.ClearHandler(r))
	err = http.ListenAndServe(":2000", nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
