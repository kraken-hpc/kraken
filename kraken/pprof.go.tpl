package main

//{{if .Pprof}}
import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go func() {
		listener, e := net.Listen("tcp", ":0")
		if e != nil {
			log.Printf("failed to start pprof: %v", e)
			return
		}
		log.Printf("pprof listening on port: %s:%d", listener.Addr().(*net.TCPAddr).IP.String(), listener.Addr().(*net.TCPAddr).Port)
		log.Println(http.Serve(listener, nil))
	}()
}

//{{end}}
