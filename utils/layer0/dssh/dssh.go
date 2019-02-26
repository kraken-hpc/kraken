/* dssh.go: dump secure shell, an sshd with no authentication
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
	gossh "golang.org/x/crypto/ssh"
)

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

const shell = "/bbin/rush"

func sshHandler(s ssh.Session) {
	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd := exec.Command(shell)
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term), "PATH=/bbin")
		f, err := pty.Start(cmd)
		if err != nil {
			log.Println(err)
		}
		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			io.Copy(f, s) // stdin
		}()
		io.Copy(s, f) // stdout
	} else {
		ucmd := s.Command()
		cmd := exec.Command(ucmd[0], ucmd[1:]...)
		comb, err := cmd.CombinedOutput()
		if err != nil {
			io.WriteString(s, err.Error())
		} else {
			io.WriteString(s, string(comb))
		}
	}
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// public key decision logic here
	return true
}

func main() {
	s := &ssh.Server{
		Addr:             ":2222",
		Handler:          sshHandler,
		PublicKeyHandler: publicKeyHandler,
	}

	// generate host key -- we create a new host key every time we start
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		log.Fatal(err)
	}
	s.AddHostKey(signer)

	log.Println("starting ssh server on port :2222")
	s.ListenAndServe()
}
