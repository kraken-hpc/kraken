/* powermanapi.go: this api provides limited powerman control through a restapi
 *
 * Author: R. Eli Snyder <resnyder@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"

	"github.com/gorilla/mux"
)

type cmdType uint8

const (
	on cmdType = iota
	off
	stat
	allstat
)

type nodeState struct {
	NodeName string `json:"name"`
	State    string `json:"state"`
}

type queueItem struct {
	Nodes []string
	Cmd   cmdType
	RChan chan []nodeState
}

type pmAPI struct {
	router *mux.Router
	srv    *http.Server
	queue  []queueItem
	mutex  *sync.Mutex
	ticker *time.Ticker
}

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

	var p pmAPI
	p.mutex = &sync.Mutex{}
	p.setupRouter()
	go p.startServer()

	dur, _ := time.ParseDuration("200ms")
	p.ticker = time.NewTicker(dur)

	for {
		select {
		case <-p.ticker.C: // send any commands that are waiting in queue
			p.mutex.Lock()
			queueLength := len(p.queue)
			p.mutex.Unlock()
			if queueLength > 0 {
				p.execute()
			}
			break
		}
	}
}

func (p *pmAPI) setupRouter() {
	p.router = mux.NewRouter()
	p.router.HandleFunc(*urlBase+"/setstate/{state}/{names}", p.setNodesHandler).Methods("GET")
	p.router.HandleFunc(*urlBase+"/nodestatus/{names}", p.nodeStatusHandler).Methods("GET")
	p.router.HandleFunc(*urlBase+"/allnodesstatus", p.allNodeStatusHandler).Methods("GET")
}

func (p *pmAPI) startServer() {
	for {
		p.srv = &http.Server{
			Handler: handlers.CORS(
				handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
				handlers.AllowedOrigins([]string{"*"}),
				handlers.AllowedMethods([]string{"GET"}),
			)(p.router),
			Addr:         fmt.Sprintf("%s:%d", *listenIP, *listenPort),
			WriteTimeout: 45 * time.Second,
			ReadTimeout:  15 * time.Second,
		}
		log.Printf("starting http service at: http://%s:%d%s\n", *listenIP, *listenPort, *urlBase)
		if e := p.srv.ListenAndServe(); e != nil {
			log.Printf("failed to start http service: %v\n", e)
		}
	}
}

func (p *pmAPI) execute() {
	var resp []nodeState
	var queueItem queueItem

	p.mutex.Lock()
	queueLength := len(p.queue)
	// Get first item in queue
	queueItem, p.queue = p.queue[0], p.queue[1:]
	p.mutex.Unlock()

	if queueLength > 1 && (queueItem.Cmd == stat || queueItem.Cmd == allstat) {
		// Ignore this stat call because we've got other items in the queue that should be executed first
		if *verbose {
			log.Printf("Skipping status request\n")
		}
		queueItem.RChan <- resp
		return
	}

	switch queueItem.Cmd {
	case on:
		fallthrough
	case off:
		var e error
		resp, e = setNodes(queueItem.Nodes, queueItem.Cmd)
		if e != nil {
			log.Printf("Error setting node state: %v\n", e)
		}
	case stat:
		fallthrough
	case allstat:
		var e error
		resp, e = nodeStatus(queueItem.Nodes)
		if e != nil {
			log.Printf("Error getting node state: %v\n", e)
		}
	default:
		log.Printf("got unrecognized command: %v\n", queueItem.Cmd)
	}

	queueItem.RChan <- resp
}

func (p *pmAPI) setNodesHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	names := strings.Split(params["names"], ",")

	var cmd cmdType

	if params["state"] == "poweron" {
		cmd = on
	} else if params["state"] == "poweroff" {
		cmd = off
	} else {
		log.Printf("Got unrecognized power state: %v. Looking for poweron or poweroff\n", params["state"])
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	responseChannel := make(chan []nodeState, 1)

	queueItem := queueItem{
		Nodes: names,
		Cmd:   cmd,
		RChan: responseChannel,
	}

	p.mutex.Lock()
	p.queue = append(p.queue, queueItem)
	p.mutex.Unlock()

	resp := <-queueItem.RChan

	if resp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(resp)
	w.Write(j)
}

func (p *pmAPI) nodeStatusHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	names := strings.Split(params["names"], ",")

	responseChannel := make(chan []nodeState, 1)

	queueItem := queueItem{
		Nodes: names,
		Cmd:   stat,
		RChan: responseChannel,
	}

	p.mutex.Lock()
	p.queue = append(p.queue, queueItem)
	p.mutex.Unlock()

	resp := <-queueItem.RChan

	if resp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(resp)
	w.Write(j)
}

func (p *pmAPI) allNodeStatusHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var emptySlice []string

	responseChannel := make(chan []nodeState, 1)

	queueItem := queueItem{
		Nodes: emptySlice,
		Cmd:   allstat,
		RChan: responseChannel,
	}

	p.mutex.Lock()
	p.queue = append(p.queue, queueItem)
	p.mutex.Unlock()

	resp := <-queueItem.RChan

	if resp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	j, _ := json.Marshal(resp)
	w.Write(j)
}

func setNodes(nodes []string, state cmdType) (resp []nodeState, e error) {
	var powerState string
	if state == on {
		powerState = "-1"
	} else if state == off {
		powerState = "-0"
	}

	cmd := exec.Command(*pmcPath, powerState, strings.Join(nodes, ","))
	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}
	e = cmd.Run()

	if e != nil {
		return
	}
	for _, nodeName := range nodes {
		var newNodeState nodeState
		newNodeState.NodeName = nodeName
		if state == on {
			newNodeState.State = "POWER_ON"
		} else if state == off {
			newNodeState.State = "POWER_OFF"
		}
		resp = append(resp, newNodeState)
	}
	return
}

func nodeStatus(nodeNames []string) (resp []nodeState, e error) {
	var cmd *exec.Cmd
	if nodeNames == nil {
		cmd = exec.Command(*pmcPath, "-q")
	} else {
		cmd = exec.Command(*pmcPath, "-Q", strings.Join(nodeNames, ","))
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if *verbose {
		log.Printf("Run: %v\n", cmd.Args)
	}
	e = cmd.Run()
	if e != nil {
		return
	}

	on, off, unknown := parsePowermanOutput(stdout.String())

	for _, nodeName := range on {
		var newNodeState nodeState
		newNodeState.NodeName = nodeName
		newNodeState.State = "POWER_ON"
		resp = append(resp, newNodeState)
	}
	for _, nodeName := range off {
		var newNodeState nodeState
		newNodeState.NodeName = nodeName
		newNodeState.State = "POWER_OFF"
		resp = append(resp, newNodeState)

	}
	for _, nodeName := range unknown {
		var newNodeState nodeState
		newNodeState.NodeName = nodeName
		newNodeState.State = "PHYS_UNKNOWN"
		resp = append(resp, newNodeState)
	}

	return
}

func parsePowermanOutput(output string) (on []string, off []string, unknown []string) {
	lines := strings.Split(output, "\n")

	// iterate through each line
	for _, line := range lines {
		if strings.Contains(line, "on:") {
			strippedLine := strings.Replace(line, "on:", "", 1)
			strippedLine = strings.ReplaceAll(strippedLine, " ", "")
			on = parseValues(strippedLine)
		} else if strings.Contains(line, "off:") {
			strippedLine := strings.Replace(line, "off:", "", 1)
			strippedLine = strings.ReplaceAll(strippedLine, " ", "")
			off = parseValues(strippedLine)
		} else if strings.Contains(line, "unknown:") {
			strippedLine := strings.Replace(line, "unknown:", "", 1)
			strippedLine = strings.ReplaceAll(strippedLine, " ", "")
			unknown = parseValues(strippedLine)
		}
	}
	return
}

func parseValues(line string) (nodes []string) {
	prefixExpression, _ := regexp.Compile("([a-zA-Z]+\\-[a-zA-Z]+|[a-zA-Z]+)")

	type prefixInfo struct {
		prefix     string
		startIndex int
		endIndex   int
	}
	var prefixes []prefixInfo

	prefixNames := prefixExpression.FindAllString(line, -1)
	prefixIndices := prefixExpression.FindAllStringIndex(line, -1)

	for _, prefixName := range prefixNames {
		var newPrefixInfo prefixInfo
		newPrefixInfo.prefix = prefixName
		prefixes = append(prefixes, newPrefixInfo)
	}

	for i, prefixIndices := range prefixIndices {
		prefixes[i].startIndex = prefixIndices[0]
		prefixes[i].endIndex = prefixIndices[1]
	}

	// for each prefix check if it's followed by a digit or a bracket
	for _, prefixInfo := range prefixes {
		if line[prefixInfo.endIndex] == '[' {
			// It's a range of nodes
			nodeRangeExpression, _ := regexp.Compile(fmt.Sprintf("%s\\[(.*?)\\]", prefixInfo.prefix))
			entireNodeRange := nodeRangeExpression.FindStringSubmatch(line)[1]
			nodeRange := parseRange(entireNodeRange)
			for _, nodeValue := range nodeRange {
				nodes = append(nodes, fmt.Sprintf("%v%v", prefixInfo.prefix, nodeValue))
			}
		} else {
			// It's a single node
			nodeValueExpression, _ := regexp.Compile(fmt.Sprintf("%s(\\d+)", prefixInfo.prefix))
			m := nodeValueExpression.FindStringSubmatch(line)
			nodeValue := ""
			if len(m) > 1 { // are there digits after the prefix or no?
				nodeValue = m[1]
			}
			nodes = append(nodes, fmt.Sprintf("%v%v", prefixInfo.prefix, nodeValue))
		}
	}

	return
}

func parseRange(input string) (rangeValues []string) {
	rangeExpression, _ := regexp.Compile("(\\d+)-(\\d+)|\\d+")
	stringSubMatches := rangeExpression.FindAllStringSubmatch(input, -1)
	for _, subMatch := range stringSubMatches {
		if strings.Contains(subMatch[0], "-") {
			// it's a range of nodes
			digitLength := len(subMatch[1])
			low, lerr := strconv.Atoi(subMatch[1])
			high, herr := strconv.Atoi(subMatch[2])
			if lerr != nil || herr != nil {
				log.Printf("Error converting low or high values of range. Low Error: %v High Error: %v\n", lerr, herr)
			}
			count := low
			for count <= high {
				// Special formatting pads any numbers with leading zeros if needed
				rangeValues = append(rangeValues, fmt.Sprintf("%0[2]*[1]d", count, digitLength))
				count++
			}
		} else {
			// it's a single node
			rangeValues = append(rangeValues, subMatch[0])
		}
	}
	return
}
