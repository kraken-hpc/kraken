/* powermanapi.go: this api provides limited powerman control through a restapi
 *
 * Author: R. Eli Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/handlers"

	"github.com/gorilla/mux"
)

var listenIP, urlBase, pmcPath *string
var listenPort *uint
var verbose *bool

func main() {
	urlBase = flag.String("base", "/powermancontrol", "base URL for api")
	pmcPath = flag.String("pmc", "powerman", "full path to powermancontrol command")
	listenIP = flag.String("ip", "127.0.0.1", "ip to listen on")
	listenPort = flag.Uint("port", 8269, "port to listen on")
	verbose = flag.Bool("v", false, "verbose messages")
	flag.Parse()

	router := mux.NewRouter()
	router.HandleFunc(*urlBase+"/poweron/{name}", powerOn).Methods("GET")
	router.HandleFunc(*urlBase+"/poweroff/{name}", powerOff).Methods("GET")
	router.HandleFunc(*urlBase+"/nodeStatus/{name}", nodeStatus).Methods("GET")

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

func powerOff(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	cmd := exec.Command(*pmcPath, "-0", params["name"])
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

func powerOn(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	cmd := exec.Command(*pmcPath, "-1", params["name"])

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

func nodeStatus(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	cmd := exec.Command(*pmcPath, "-Q", params["name"])
	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		log.Printf("Error running the nodeDiscover command: %v\n", err)
		return
	}

	discOut := strings.Split(stdout.String(), "\n")
	if len(discOut) != 4 {
		log.Printf("Unexpected length for stdout in nodeDiscover: %d", len(discOut))
		return
	}

	var rs struct {
		State string
	}

	if strings.Contains(discOut[0], params["name"]) {
		rs.State = "POWER_ON"
	} else if strings.Contains(discOut[1], params["name"]) {
		rs.State = "POWER_OFF"
	} else if strings.Contains(discOut[2], params["name"]) {
		rs.State = "PHYS_UNKNOWN"
	} else {
		log.Printf("Node not found in powerman discovery: %s", params["name"])
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	if *verbose {
		log.Print("Status: %s\n", stdout.String())
	}
	j, _ := json.Marshal(rs)
	if *verbose {
		log.Printf("%s\n", j)
	}
	w.Write(j)
}
