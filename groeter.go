// groeter
// HTTP Server using github.com/hduplooy/groet for routing
// The config file is provided as the first parameter when the executable is started
package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/hduplooy/groet"
)

// WebJson reflects the entries in the configuration file
// Type - can be path, exact, match, port, protocol, host, domain, any -- see groet for what it means
// Match - the matching value used for the specific type
// StripPrefix - must the prefix (the Match) be stripped from the path when passed on to the action
// Action - can be fileserver, reverseproxy, subrouter
// Value - is path for fileserver or url for reverseproxy or nil for subrouter
// Router - is the entries for the subrouter for this entry
type WebJson struct {
	Type        string    `json:"type"`
	Match       string    `json:"match"`
	StripPrefix bool      `json:"stripprefix"`
	Action      string    `json:"action"`
	Value       string    `json:"value"`
	Router      []WebJson `json:"router"`
}

// processEntries will go through all the entries and call the appropriate groet functions
// to add the necessary entries to the route
// All the necessary values are taken from the WebJson entries
// Return the Router when finished
func processEntries(ents []WebJson) *groet.Router {
	rt := groet.NewRouter()
	for _, val := range ents {
		var ent *groet.RouterEntry
		switch val.Type {
		case "path":
			ent = rt.Path(val.Match)
		case "exact":
			ent = rt.PathExact(val.Match)
		case "domain":
			ent = rt.Domain(val.Match)
		case "port":
			ent = rt.Port(val.Match)
		case "protocol":
			ent = rt.Protocol(val.Match)
		case "host":
			ent = rt.Host(val.Match)
		case "match":
			ent = rt.Match(val.Match)
		case "any":
			ent = rt.Any()
		}
		switch val.Action {
		case "fileserver":
			if val.StripPrefix {
				ent.Handle(http.StripPrefix("/"+val.Match, http.FileServer(http.Dir(val.Value))))
			} else {
				ent.Handle(http.FileServer(http.Dir(val.Value)))
			}
		case "reverseproxy":
			url, err := url.Parse(val.Value)
			if err == nil {
				if val.StripPrefix {
					ent.Handle(http.StripPrefix("/"+val.Match, httputil.NewSingleHostReverseProxy(url)))
				} else {
					ent.Handle(httputil.NewSingleHostReverseProxy(url))
				}
			} else {
				log.Printf("URL %s for match %s does not parse!: %s", val.Value, val.Match, err.Error())
				os.Exit(-1)
			}
		case "subrouter":
			ent.Subrouter(processEntries(val.Router))
		}
	}
	return rt
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("No config file specified!")
		os.Exit(-1)
	}
	buf, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("Error reading config file %s!: %s", os.Args[1], err.Error())
		os.Exit(-1)
	}
	var ents []WebJson
	err = json.Unmarshal(buf, &ents)
	if err != nil {
		log.Printf("Error parsing config file!: %s", err.Error())
		os.Exit(-1)
	}
	root := processEntries(ents)
	http.ListenAndServe(":8080", root)
}
