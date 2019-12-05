/* uinit.go: a simple init launcher for kraken layer0
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	re "regexp"
	"strconv"
	"syscall"
)

// info about background processes, inclding std{out,in,err} will be placed here
const ioDir = "/tmp/io"

// perm mask for ioDir files
const ioMode = 0700

type command struct {
	Cmd        string
	Args       []string
	Background bool
}

var myIP string
var myNet string
var myID string
var myParent string

const (
	kernArgFile = "/proc/cmdline"
	moduleFile  = "/modules.txt"
	insmodCmd   = "/bbin/insmod"
)

// loadModules doesn't actually load the modules.  It just creates a list of commands to get executed
// it reads moduleFile, and creates a list of insmod commands in order
func loadModules() (cmds []command, e error) {
	var f *os.File
	if _, e = os.Stat(moduleFile); os.IsNotExist(e) {
		fmt.Printf("%s does not exist, we will not load modules\n", moduleFile)
		return
	}
	if f, e = os.Open(moduleFile); e != nil {
		fmt.Printf("failed to open module file, %s: %v\n", moduleFile, e)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Printf("adding module to load list: %s\n", scanner.Text())
		cmd := command{
			Cmd:        insmodCmd,
			Args:       []string{insmodCmd, scanner.Text()},
			Background: false,
		}
		cmds = append(cmds, cmd)
	}
	return
}

func goExec(cmd *exec.Cmd) {
	if err := cmd.Run(); err != nil {
		log.Printf("command %s failed: %s\n", cmd.Path, err)
	}
}

func kernArgs(file string) {
	d, e := ioutil.ReadFile(file)
	if e != nil {
		return
	}
	reIP := re.MustCompile("kraken.ip=(([0-9,\\.])*)")
	reNet := re.MustCompile("kraken.net=(([0-9,\\.])*)")
	reID := re.MustCompile("kraken.id=([0-9a-f\\-]*)")
	reParent := re.MustCompile("kraken.parent=(([0-9,\\.])*)")

	// This will fail badly if we don't have commandline arguments set properly

	m := reIP.FindAllStringSubmatch(string(d), -1)
	if len(m) >= 1 && len(m[0]) >= 2 {
		myIP = m[0][1]
	} else {
		log.Fatalf("Could not get kraken.ip=?, %v\n", m)
	}
	m = reID.FindAllStringSubmatch(string(d), -1)
	if len(m) >= 1 && len(m[0]) >= 2 {
		myID = m[0][1]
	} else {
		log.Fatalf("Could not get kraken.id=?, %v\n", m)
	}
	m = reNet.FindAllStringSubmatch(string(d), -1)
	if len(m) >= 1 && len(m[0]) >= 2 {
		myNet = m[0][1]
	} else {
		log.Fatalf("Could not get kraken.net=?, %v\n", m)
	}
	m = reParent.FindAllStringSubmatch(string(d), -1)
	if len(m) >= 1 && len(m[0]) >= 2 {
		myParent = m[0][1]
	} else {
		log.Fatalf("Could not get kraken.parent=?, %v\n", m)
	}

	log.Printf("ID: %s IP: %s Net: %s Parent %s\n", myID, myIP, myNet, myParent)
	return
}

func main() {
	log.Println("starting uinit")
	kernArgs(kernArgFile)

	// This defines what will run
	var cmdList = []command{
		// give the system 2 seconds to come to its senses
		// shouldn't be necessary, but seems to help
		{
			Cmd:        "/bbin/sleep",
			Background: false,
			Args:       []string{"/bbin/sleep", "2"},
		},
		/* we don't use dhcp with kraken
		command{
			Cmd:        "/bbin/dhclient",
			Background: false,
			Args:       []string{"/bbin/dhclient", "eth0"},
		},
		*/
		{
			Cmd:        "/bbin/ip",
			Background: false,
			Args:       []string{"/bbin/ip", "addr", "add", myIP + "/" + myNet, "dev", "eth0"},
		},
		{
			Cmd:        "/bbin/ip",
			Background: false,
			Args:       []string{"/bbin/ip", "link", "set", "eth0", "up"},
		},
		{
			Cmd:        "/bbin/sshd",
			Background: true,
		},
		{
			Cmd:        "/bin/kraken",
			Background: true,
			Args:       []string{"/bin/kraken", "-ip", myIP, "-parent", myParent, "-id", myID},
		},
		{
			Cmd:        "/bin/RFEmulator-pull",
			Background: true,
		},
		{
			Cmd:        "/bbin/elvish",
			Background: false,
		},
	}

	modCmds, _ := loadModules()

	cmdList = append(modCmds, cmdList...)

	envs := os.Environ()
	os.MkdirAll(ioDir, os.ModeDir&ioMode)
	var cmds []*exec.Cmd
	for i, c := range cmdList {
		v := c.Cmd
		if _, err := os.Stat(v); !os.IsNotExist(err) {
			cmd := exec.Command(v)
			cmd.Dir = "/"
			cmd.Env = envs
			log.Printf("cmd: %d/%d %s", i+1, len(cmdList), cmd.Path)
			if c.Background {
				cmdIODir := ioDir + "/" + strconv.Itoa(i)
				if err = os.Mkdir(cmdIODir, os.ModeDir&ioMode); err != nil {
					log.Println(err)
				}
				if err = ioutil.WriteFile(cmdIODir+"/"+"cmd", []byte(v), os.ModePerm&ioMode); err != nil {
					log.Println(err)
				}
				stdin, err := os.OpenFile(cmdIODir+"/"+"stdin", os.O_RDONLY|os.O_CREATE, ioMode&os.ModePerm)
				if err != nil {
					log.Println(err)
				}
				stdout, err := os.OpenFile(cmdIODir+"/"+"stdout", os.O_WRONLY|os.O_CREATE, ioMode&os.ModePerm)
				if err != nil {
					log.Println(err)
				}
				stderr, err := os.OpenFile(cmdIODir+"/"+"stderr", os.O_WRONLY|os.O_CREATE, ioMode&os.ModePerm)
				if err != nil {
					log.Println(err)
				}
				defer stdin.Close()
				defer stdout.Close()
				defer stderr.Close()
				cmd.Stdin = stdin
				cmd.Stdout = stdout
				cmd.Stderr = stderr
				cmd.Args = c.Args
				go goExec(cmd)
				cmds = append(cmds, cmd)
			} else {
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
				cmd.Args = c.Args
				if err := cmd.Run(); err != nil {
					log.Printf("command %s failed: %s", cmd.Path, err)
				}
			}
		}
	}
	log.Println("uinit exit")
}
