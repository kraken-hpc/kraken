/* inito.go: a simple init launcher for kraken layer0
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Los Alamos National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
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

// This defines what will run
var cmdList = []command{
	// give the system 2 seconds to come to its senses
	// shouldn't be necessary, but seems to help
	command{
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
	command{
		Cmd:        "/bin/dssh",
		Background: true,
	},
	command{
		Cmd:        "/bin/kraken",
		Background: true,
		Args:       []string{"/bin/kraken", "-ip", myIP, "-parent", myParent, "-id", myID},
	},
	command{
		Cmd:        "/bbin/rush",
		Background: false,
	},
}

var myIP string
var myNet string
var myID string
var myParent string

const kernArgFile = "/proc/cmdline"

func goExec(cmd *exec.Cmd) {
	if err := cmd.Run(); err != nil {
		log.Printf("command %s failed: %s", cmd.Path, err)
	}
}

func kernArgs() {
	d, e := ioutil.ReadFile(kernArgFile)
	if e != nil {
		return
	}
	reIP := re.MustCompile("kraken_ip=(([0-9,\\.])*)")
	reNet := re.MustCompile("kraken_net=(([0-9,\\.])*)")
	reID := re.MustCompile("kraken_id=([0-9a-f\\-]*)")
	reParent := re.MustCompile("kraken_parent=(([0-9,\\.])*)")

	// This will fail badly if we don't have commandline arguments set properly
	myIP = reIP.FindAllStringSubmatch(string(d), -1)[0][1]
	myID = reID.FindAllStringSubmatch(string(d), -1)[0][1]
	myNet = reNet.FindAllStringSubmatch(string(d), -1)[0][1]
	myParent = reParent.FindAllStringSubmatch(string(d), -1)[0][1]

	return
}

func main() {
	log.Println("starting uinit")
	kernArgs()
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
