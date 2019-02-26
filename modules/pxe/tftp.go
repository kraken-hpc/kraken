/* tftp.go: provides TFTP support for pxe
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package pxe

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hpc/kraken/extensions/IPv4"
	"github.com/hpc/kraken/lib"
	"github.com/pin/tftp"
)

// StartTFTP starts up the TFTP service
func (px *PXE) StartTFTP(ip net.IP) {
	px.api.Log(lib.LLNOTICE, "starting TFTP service")
	srv := tftp.NewServer(px.writeToTFTP, nil)
	e := srv.ListenAndServe(ip.String() + ":69")
	if e != nil {
		px.api.Logf(lib.LLCRITICAL, "TFTP failed to start: %v", e)
	}
	px.api.Log(lib.LLNOTICE, "TFTP service stopped")
}

func (px *PXE) writeToTFTP(filename string, rf io.ReaderFrom) (e error) {
	ip := rf.(tftp.OutgoingTransfer).RemoteAddr().IP
	n := px.NodeGet(queryByIP, ip.String())
	if n == nil {
		px.api.Logf(lib.LLDEBUG, "got TFTP request from unknown node: %s", ip.String())
		return fmt.Errorf("got TFTP request from unknown node: %s", ip.String())
	}
	vs := n.GetValues([]string{"/Arch", "/Platform"})
	lfile := filepath.Join(
		px.cfg.TftpDir,
		vs["/Arch"].String(),
		vs["/Platform"].String(),
		filename,
	)
	var f io.Reader
	if _, e = os.Stat(lfile); os.IsNotExist(e) {
		if _, e = os.Stat(lfile + ".tpl"); os.IsNotExist(e) {
			// neither file nor template exist
			px.api.Logf(lib.LLDEBUG, "no such file: %s", lfile)
			return fmt.Errorf("no such file: %s", lfile)
		}
		// file doesn't exist, but template does
		// we could potentially make a lot more data than this available
		type tplData struct {
			IP       string
			CIDR     string
			ID       string
			ParentIP string
		}
		data := tplData{}
		i, _ := n.GetValue(px.cfg.IpUrl)
		data.IP = IPv4.BytesToIP(i.Bytes()).String()
		i, _ = n.GetValue(px.cfg.NmUrl)
		subip := IPv4.BytesToIP(i.Bytes())
		cidr, _ := net.IPMask(subip.To4()).Size()
		data.CIDR = strconv.Itoa(cidr)
		data.ID = n.ID().String()
		data.ParentIP = px.selfIP.String()
		tpl, e := template.ParseFiles(lfile + ".tpl")
		if e != nil {
			px.api.Logf(lib.LLDEBUG, "template parse error: %v", e)
			return fmt.Errorf("template parse error: %v", e)
		}
		f = &bytes.Buffer{}
		tpl.Execute(f.(io.Writer), &data)
	} else {
		// file exists
		f, e = os.Open(lfile)
		defer f.(*os.File).Close()
	}

	written, e := rf.ReadFrom(f)
	px.api.Logf(lib.LLDEBUG, "wrote %s (%s), %d bytes", filename, lfile, written)
	return
}
