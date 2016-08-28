// groeter
// HTTP Server using github.com/hduplooy/groet for routing
// The config file is provided as the first parameter when the executable is started
package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cgi"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hduplooy/groet"
)

// WebConfig describes the config file
// CGI is all the cgi declarations
// Router is all router declarations at the top level
type WebConfig struct {
	CGI    []WebCGI    `json:"cgi"`
	Router []WebRouter `json:"router"`
}

// WebGGI is a cgi declaration to let some other program like PHP,Perl,Python etc. handle the request
// Ext is the file extension
// Program is the program name to call
// Path is the actual path to the program to call
type WebCGI struct {
	Ext     string `json:"ext"`
	Program string `json:"program"`
	Path    string `json:"-"`
}

// WebRouter reflects the top level router entries in the configuration file
// Type - can be path, exact, match, port, protocol, host, domain, any -- see groet for what it means
// Match - the matching value used for the specific type
// StripPrefix - must the prefix (the Match) be stripped from the path when passed on to the action
// Action - can be fileserver, reverseproxy, subrouter
// Value - is path for fileserver or url for reverseproxy or nil for subrouter
// Router - is the entries for the subrouter for this entry
type WebRouter struct {
	Type        string      `json:"type"`
	Match       string      `json:"match"`
	StripPrefix bool        `json:"stripprefix"`
	Action      string      `json:"action"`
	Value       string      `json:"value"`
	Router      []WebRouter `json:"router"`
}

// processConig will first process the CGI declarations
// then the Router entries are processed and the router is returned
func processConfig(config WebConfig) *groet.Router {
	processCGIEntries(config.CGI)
	return processEntries(config.Router)
}

// processCGIEntries go through the cgi declarations and
//    lookup the path to the actual program and save them in the cgimap for use
func processCGIEntries(ents []WebCGI) {
	cgimap = make(map[string]WebCGI)
	for _, ent := range ents {
		ent.Path, _ = exec.LookPath(ent.Program)
		cgimap[ent.Ext] = ent
	}
}

// processEntries will go through all the entries and call the appropriate groet functions
// to add the necessary entries to the route
// All the necessary values are taken from the WebRouter entries
// Return the Router when finished
func processEntries(ents []WebRouter) *groet.Router {
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
				ent.Handle(http.StripPrefix("/"+val.Match+"/", FileServer(val.Value)))
			} else {
				ent.Handle(FileServer(val.Value))
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

// cgimap keeps all the cgi declaration for lookup
var cgimap map[string]WebCGI

// FileHandler is our filehandler
// Root holds the directory path to the root of where our files (and cgi programs) reside
type FileHandler struct {
	Root string
}

// FileHandler.ServeHTTP handles requests accordingly
// If the extension is one of the cgi entries then call that cgi handler
// if it is a directory then first index.html is searched and then index for every cgi extension
//    if any is found then that is called
// if it is not a directory the file is served if it exists
// if none of above NotFoundHandler is used
func (fh FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// the actual file name
	fname := fh.Root + "/" + r.URL.Path
	ext := filepath.Ext(fname) // Get the extension of the file
	if len(ext) > 0 {          // remove leading "." if it is and extension
		ext = ext[1:]
	}
	_, ok := cgimap[ext]
	if ok { // If it is a cgi extension then get a cgi handler and serve the request to it
		fh.cgiHandler(fname).ServeHTTP(w, r)
		return
	}
	// Let's see if the file exists
	st, err := os.Stat(fname)
	if err != nil { // If it is not a file or directory, then not found
		http.NotFoundHandler().ServeHTTP(w, r)
	} else {
		if st.IsDir() { // If it is a directory search for and index file
			if fname[len(fname)-1] != '/' { // Add a trailing / if none
				fname += "/"
			}
			// If index.html exists within the directory then serve it
			if st, err = os.Stat(fname + "index.html"); err == nil && !st.IsDir() {
				http.ServeFile(w, r, fname+"index.html")
			} else {
				// For every cgi extension see if that index file exists
				//   if it does then let pass it on to the cgi handler
				for key, _ := range cgimap {
					if st, err = os.Stat(fname + "index." + key); err == nil && !st.IsDir() {
						fh.cgiHandler(fname+"index."+key).ServeHTTP(w, r)
						return
					}
				}
				// If nothing matches then do a not found (we don't do directory listings)
				http.NotFoundHandler().ServeHTTP(w, r)
			}
		} else {
			// If it is a file then serve it
			http.ServeFile(w, r, fname)
		}
	}
}

// cgiHandler create the cgi.Handler to handle the specific extension
func (fh FileHandler) cgiHandler(file string) *cgi.Handler {
	ext := filepath.Ext(file)[1:] // Get proper extension
	prog := cgimap[ext]           // Get entry that was defined
	return &cgi.Handler{
		Path: prog.Path, // Path to actual program to execute
		Root: fh.Root,   // Root used when running the program
		Dir:  fh.Root,
		Args: []string{file},                      // The argument passed is just the file that the url is pointing to
		Env:  []string{"SCRIPT_FILENAME=" + file}, // Set the CGI environment variable
	}
}

// FileServer return a FileHandler
func FileServer(path string) FileHandler {
	return FileHandler{Root: path}
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("No config file specified!")
		os.Exit(-1)
	}
	// Read the config file into buffer
	buf, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("Error reading config file %s!: %s", os.Args[1], err.Error())
		os.Exit(-1)
	}
	// Populate structure based on json config file
	var config WebConfig
	err = json.Unmarshal(buf, &config)
	if err != nil {
		log.Printf("Error parsing config file!: %s", err.Error())
		os.Exit(-1)
	}
	// Process the config file and return the main router entries
	root := processConfig(config)
	// Start the server
	http.ListenAndServe(":8080", root)
}
