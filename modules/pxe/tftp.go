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

	"github.com/hpc/kraken/extensions/ipv4"
	"github.com/hpc/kraken/lib/types"
	"github.com/pin/tftp"
)

// StartTFTP starts up the TFTP service
func (px *PXE) StartTFTP(ip net.IP) {
	px.api.Log(types.LLNOTICE, "starting TFTP service")
	srv := tftp.NewServer(px.writeToTFTP, nil)
	e := srv.ListenAndServe(ip.String() + ":69")
	if e != nil {
		px.api.Logf(types.LLCRITICAL, "TFTP failed to start: %v", e)
	}
	px.api.Log(types.LLNOTICE, "TFTP service stopped")
}

func (px *PXE) writeToTFTP(filename string, rf io.ReaderFrom) (e error) {
	ip := rf.(tftp.OutgoingTransfer).RemoteAddr().IP
	n := px.NodeGet(queryByIP, ip.String())
	if n == nil {
		px.api.Logf(types.LLDEBUG, "got TFTP request from unknown node: %s", ip.String())
		return fmt.Errorf("got TFTP request from unknown node: %s", ip.String())
	}
	vs, e := n.GetValues([]string{"/Arch", "/Platform"})
	if e != nil {
		px.api.Logf(types.LLERROR, "error getting values for node: %v", e)
	}
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
			px.api.Logf(types.LLDEBUG, "no such file: %s", lfile)
			return fmt.Errorf("no such file: %s", lfile)
		}
		// file doesn't exist, but template does
		// we could potentially make a lot more data than this available
		// this is the basic stuff that's needed to get linked up with the parent
		type tplData struct {
			Iface    string
			IP       string
			CIDR     string
			ID       string
			ParentIP string
		}
		data := tplData{}
		iface, _ := n.GetValue(px.cfg.SrvIfaceUrl)
		data.Iface = iface.String()
		i, _ := n.GetValue(px.cfg.IpUrl)
		data.IP = i.Interface().(*ipv4.IP).String()
		i, _ = n.GetValue(px.cfg.NmUrl)
		subip := i.Interface().(*ipv4.IP)
		cidr, _ := net.IPMask(subip.To4()).Size()
		data.CIDR = strconv.Itoa(cidr)
		data.ID = n.ID().String()
		data.ParentIP = px.selfIP.String()
		tpl, e := template.ParseFiles(lfile + ".tpl")
		if e != nil {
			px.api.Logf(types.LLDEBUG, "template parse error: %v", e)
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
	px.api.Logf(types.LLDEBUG, "wrote %s (%s), %d bytes", filename, lfile, written)
	return
}
