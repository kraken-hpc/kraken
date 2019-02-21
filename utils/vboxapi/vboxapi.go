/* vboxapi.go: this api provides limited virtualbox control through a restapi
 *				the api mimicks a limited version of vboxmanage-rest-api
 *				(https://www.npmjs.com/package/vboxmanage-rest-api)
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/handlers"

	"github.com/gorilla/mux"
)

var listenIP, urlBase, vbmPath *string
var listenPort *uint
var verbose *bool

type vbmResponse struct {
	Err    int32    `json:"e,omitempty"`
	ErrMsg string   `json:"err_msg,omitempty"`
	Off    []uint32 `json:"off,omitempty"`
	On     []uint32 `json:"on,omitempty"`
}

func main() {
	urlBase = flag.String("base", "/vboxmanage", "base URL for api")
	vbmPath = flag.String("vbm", "/usr/local/bin/vboxmanage", "full path to vboxmanage command")
	listenIP = flag.String("ip", "127.0.0.1", "ip to listen on")
	listenPort = flag.Uint("port", 8269, "port to listen on")
	verbose = flag.Bool("v", false, "verbose messages")
	flag.Parse()

	router := mux.NewRouter()
	router.HandleFunc(*urlBase+"/showvminfo/{name}", showVMInfo).Methods("GET")
	router.HandleFunc(*urlBase+"/controlvm/{name}/poweroff", powerOff).Methods("GET")
	router.HandleFunc(*urlBase+"/startvm/{name}", startVM).Methods("GET")
	router.HandleFunc(*urlBase+"/startvm/{name}", startVM).Methods("GET").Queries("type", "{type}")

	srv := &http.Server{
		Handler: handlers.CORS(
			handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"GET"}),
		)(router),
		Addr:         fmt.Sprintf("%s:%d", *listenIP, *listenPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("starting http service at: http://%s:%d%s", *listenIP, *listenPort, *urlBase)
	if e := srv.ListenAndServe(); e != nil {
		log.Printf("failed to start http service: %v", e)
		return
	}
}

func showVMInfo(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	cmd := exec.Command(*vbmPath, "showvminfo", params["name"])
	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}
	co, e := cmd.Output()
	if e != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// parse out values
	var rs struct {
		Name  string
		Uuid  string
		Ram   string
		Vram  string
		State string
	}
	ret := map[string]string{
		"Name":        "",
		"UUID":        "",
		"Memory size": "",
		"VRAM size":   "",
		"State":       "",
	}
	re, _ := regexp.Compile("^([^:]*): ([^(]*)(?: \\(.*)?$")
	read := bytes.NewReader(co)
	scan := bufio.NewScanner(read)
	for scan.Scan() {
		m := re.FindStringSubmatch(scan.Text())
		if m == nil {
			continue
		}
		if len(m) < 3 {
			continue
		}
		ret[m[1]] = m[2]
	}
	rs.State = strings.TrimSpace(ret["State"])
	rs.Name = strings.TrimSpace(ret["Name"])
	rs.Uuid = strings.TrimSpace(ret["UUID"])
	rs.Ram = strings.TrimSpace(ret["Memory size"])
	rs.Vram = strings.TrimSpace(ret["VRAM size"])

	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(rs)
	if *verbose {
		log.Printf("%s\n", j)
	}
	w.Write(j)
}

func powerOff(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	cmd := exec.Command(*vbmPath, "controlvm", params["name"], "poweroff")
	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}
	co, e := cmd.CombinedOutput()
	var rs struct {
		Shell struct {
			Command   string
			Directory string
			ExitCode  int
			Output    string
		}
	}
	rs.Shell.Command = strings.Join(cmd.Args, " ")
	rs.Shell.Directory = "NOT_IMPLEMENTED"
	if e != nil {
		rs.Shell.ExitCode = 1
		rs.Shell.Output = string(co)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(rs)
	if *verbose {
		log.Printf("%s\n", j)
	}
	w.Write(j)
}

func startVM(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	guiType := req.FormValue("type")
	args := []string{"startvm", params["name"]}
	if guiType != "" {
		args = append(args, "--type", guiType)
	}
	cmd := exec.Command(*vbmPath, args...)
	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}
	co, e := cmd.CombinedOutput()
	var rs struct {
		Shell struct {
			Command   string
			Directory string
			ExitCode  int
			Output    string
		}
	}
	rs.Shell.Command = strings.Join(cmd.Args, " ")
	rs.Shell.Directory = "NOT_IMPLEMENTED"
	if e != nil {
		rs.Shell.ExitCode = 1
		rs.Shell.Output = string(co)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(rs)
	if *verbose {
		log.Printf("%s\n", j)
	}
	w.Write(j)
}
