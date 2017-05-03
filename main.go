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
	"strconv"
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
	ExpectEnv      string
}

var cfg config

type remoteUserType int

const remoteUser remoteUserType = 0

// Response to a key status request
type loadersReply struct {
	Loaders []mig.LoaderEntry `json:"loaders"`
}

// Payload submitted for a new key request
type newkeyRequest struct {
	SlotID string `json:"slot"`
}

func (n *newkeyRequest) validate() error {
	return nil
}

type requestDetails struct {
	remoteUser string
	loaders    []mig.LoaderEntry
}

func (r *requestDetails) searchUserString() string {
	return "migss-" + r.remoteUser + "-%"
}

func (r *requestDetails) convertSlotID(slotid string) (string, error) {
	ret := "migss-" + r.remoteUser + "-"
	sv := strings.Replace(slotid, "slot", "", 1)
	svint, err := strconv.ParseInt(sv, 10, 64)
	if err != nil {
		return "", err
	}
	if (svint < 1) || (svint > 3) {
		return "", fmt.Errorf("invalid slot id")
	}
	ret = ret + sv
	return ret, nil
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
		return
	}
	mp, err := renderMainPage(rdetails)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	fmt.Fprint(rw, mp)
}

func handleKeyStatus(rw http.ResponseWriter, req *http.Request) {
	rdetails, err := newRequestDetails(req)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	cli, err := newMIGClient()
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	err = rdetails.addKeys(cli)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	resp := loadersReply{}
	resp.Loaders = rdetails.loaders
	if resp.Loaders == nil {
		resp.Loaders = make([]mig.LoaderEntry, 0)
	}
	buf, err := json.Marshal(&resp)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	fmt.Fprint(rw, string(buf))
}

func handleNewKey(rw http.ResponseWriter, req *http.Request) {
	var (
		newkey newkeyRequest
		le     mig.LoaderEntry
	)

	rdetails, err := newRequestDetails(req)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()
	err = decoder.Decode(&newkey)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}

	cli, err := newMIGClient()
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	// Add any existing loader entries for this user to rdetails
	err = rdetails.addKeys(cli)

	le.Name, err = rdetails.convertSlotID(newkey.SlotID)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	le.ExpectEnv = cfg.ExpectEnv

	// At this point the loader is ready to be created, but first check and see if an
	// entry for this slot already exists. If so we will enable and rekey this entry
	// rather than create it.
	var newle mig.LoaderEntry
	found := false
	for _, x := range rdetails.loaders {
		if x.Name == le.Name {
			newle = x
			found = true
			break
		}
	}
	if found {
		err = cli.LoaderEntryStatus(newle, true)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}
		newle, err = cli.LoaderEntryKey(newle)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}
	} else {
		newle, err = cli.PostNewLoader(le)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}
		// Also enable the new loader entry
		err = cli.LoaderEntryStatus(newle, true)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			return
		}
	}
	buf, err := json.Marshal(&newle)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	fmt.Fprint(rw, string(buf))
}

func handleDelKey(rw http.ResponseWriter, req *http.Request) {
	var (
		newkey newkeyRequest
		le     mig.LoaderEntry
	)

	rdetails, err := newRequestDetails(req)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()
	err = decoder.Decode(&newkey)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	cli, err := newMIGClient()
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	err = rdetails.addKeys(cli)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	le.Name, err = rdetails.convertSlotID(newkey.SlotID)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	// We need the loader ID to change the status of the entry, locate the ID
	// in rdetails based on our loader name and add it to the request
	found := false
	for _, x := range rdetails.loaders {
		if x.Name == le.Name {
			le.ID = x.ID
			found = true
			break
		}
	}
	if !found {
		http.Error(rw, "unable to locate loader ID for slot", 500)
		return
	}
	err = cli.LoaderEntryStatus(le, false)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
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
		ru := r.Header.Get("REMOTE_USER")
		if ru == "" {
			http.Error(w, "invalid header configuration", 500)
			return
		}
		context.Set(r, remoteUser, ru)
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
	r.HandleFunc("/newkey", setContext(handleNewKey)).Methods("POST")
	r.HandleFunc("/delkey", setContext(handleDelKey)).Methods("POST")

	sp := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.PathPrefix("/static").Handler(sp)

	http.Handle("/", context.ClearHandler(r))
	err = http.ListenAndServe(":2000", nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
