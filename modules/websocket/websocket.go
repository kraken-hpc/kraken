/* restapi.go: this module provides a simple ReST API for Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/restapi.proto

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	cpb "github.com/hpc/kraken/core/proto"
	pb "github.com/hpc/kraken/modules/websocket/proto"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib"
)

var _ lib.Module = (*WebSocket)(nil)
var _ lib.ModuleSelfService = (*WebSocket)(nil)
var _ lib.ModuleWithConfig = (*WebSocket)(nil)
var _ lib.ModuleWithAllEvents = (*WebSocket)(nil)
var _ lib.ModuleWithDiscovery = (*WebSocket)(nil)

const WsStateURL = "/Services/websocket/State"

type Command uint8

const (
	SUBSCRIBE Command = iota
	UNSUBSCRIBE
)

type WebSocket struct {
	cfg    *pb.WebSocketConfig
	api    lib.APIClient
	router *mux.Router
	srv    *http.Server
	echan  <-chan lib.Event
	dchan  chan<- lib.Event
	hub    *Hub
	ticker *time.Ticker
	mutex  *sync.Mutex
	queue  []*Payload
	srvIp  net.IP
}

type Hub struct {
	clients    map[*Client]bool // Registered clients.
	broadcast  chan []*Payload  // Messages from the event stream.
	action     chan *Action     // Messages from the websocket Clients.
	register   chan *Client     // Register requests from the clients.
	unregister chan *Client     // Unregister requests from clients.
	api        lib.APIClient
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn        // The websocket connection.
	send chan []*Payload        // Buffered channel of outbound messages.
	w    *WebSocket             // Web Socket
	subs map[lib.EventType]bool // List of event types that client is subscribed to
}

type Payload struct {
	Type   lib.EventType `json:"type"`
	URL    string        `json:"url"`
	Data   string        `json:"data"`
	NodeId string        `json:"nodeid"`
	Value  string        `json:"value"`
}

func (p *Payload) String() string {
	return fmt.Sprintf("Type: %v, Url: %v, Data: %v, NodeId: %v, Value: %v", lib.EventTypeString[p.Type], p.URL, p.Data, p.NodeId, p.Value)
}

type Action struct {
	Command   Command `json:"command"`
	EventType string  `json:"type"`
	Client    *Client
}

func (w *WebSocket) Entry() {
	w.api.Logf(lib.LLDDDEBUG, "Starting entry function, %v", w.cfg)
	dur, _ := time.ParseDuration(w.cfg.GetTick())
	w.api.Logf(lib.LLDEBUG, "tick: %v, duration: %v", w.cfg.GetTick(), dur)

	nself, _ := w.api.QueryRead(w.api.Self().String())

	rAddr, e := nself.GetValue("/Services/restapi/Config/Addr")
	if e != nil {
		w.api.Logf(lib.LLERROR, "error getting restapi address")
	}
	w.api.Logf(lib.LLDDDEBUG, "setting server ip: %v", rAddr.String())
	w.srvIp = net.ParseIP(rAddr.String())

	w.api.Logf(lib.LLDDDEBUG, "setting up router")
	w.setupRouter()
	w.api.Logf(lib.LLDDDEBUG, "starting webserver")
	go w.startServer()
	w.hub = w.newHub()
	w.api.Logf(lib.LLDDDEBUG, "starting hub")
	go w.hub.run()

	w.api.Logf(lib.LLDDDEBUG, "sending discovery event")
	url := lib.NodeURLJoin(w.api.Self().String(),
		lib.URLPush(lib.URLPush("/Services", "websocket"), "State"))
	ev := core.NewEvent(
		lib.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	w.dchan <- ev

	w.api.Logf(lib.LLDDDEBUG, "starting main loop")
	for {
		// create a timer that will send queued websocket messages
		dur, _ := time.ParseDuration(w.cfg.GetTick())
		w.ticker = time.NewTicker(dur)
		select {
		case <-w.ticker.C:
			go w.sendWSMessages()
			break
		case e := <-w.echan: // event
			go w.handleEvent(e)
			break
		}
	}
}

func (w *WebSocket) handleEvent(ev lib.Event) {
	var payload = &Payload{}
	nodeID, url := lib.NodeURLSplit(ev.URL())
	switch ev.Type() {
	case lib.Event_STATE_MUTATION:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.MutationEvent).String(),
			NodeId: nodeID,
			Value:  ev.Data().(*core.MutationEvent).Mutation[1],
		}
	case lib.Event_STATE_CHANGE:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.StateChangeEvent).String(),
			NodeId: nodeID,
			Value:  lib.ValueToString(ev.Data().(*core.StateChangeEvent).Value),
		}
	case lib.Event_DISCOVERY:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.DiscoveryEvent).String(),
			NodeId: nodeID,
			Value:  ev.Data().(*core.DiscoveryEvent).ValueID,
		}
	default:
		w.api.Logf(lib.LLDEBUG, "got unknown event: %+v\n", ev.Data())
	}
	w.mutex.Lock()
	w.queue = append(w.queue, payload)
	w.mutex.Unlock()
}

func (w *WebSocket) sendWSMessages() {
	w.mutex.Lock()
	if len(w.queue) > 0 {
		w.hub.broadcast <- w.queue
		w.queue = []*Payload{}
	}
	w.mutex.Unlock()
}

func (w *WebSocket) Stop() { os.Exit(0) }

func (w *WebSocket) Name() string { return "github.com/hpc/kraken/modules/websocket" }

// SetEventsChan sets the event channel
// this is generally done by the API
func (w *WebSocket) SetEventsChan(c <-chan lib.Event) { w.echan = c }

func (w *WebSocket) SetDiscoveryChan(c chan<- lib.Event) { w.dchan = c }

func (w *WebSocket) UpdateConfig(cfg proto.Message) (e error) {
	if wc, ok := cfg.(*pb.WebSocketConfig); ok {
		w.api.Logf(lib.LLDEBUG, "updating config for websocket: %v", wc)
		w.cfg = wc
		if w.srv != nil {
			w.srvStop() // we just stop, entry will (re)start
		}
		return
	}
	return fmt.Errorf("wrong config type")
}

func (w *WebSocket) Init(api lib.APIClient) {
	w.api = api
	w.cfg = w.NewConfig().(*pb.WebSocketConfig)
	w.mutex = &sync.Mutex{}
	w.queue = []*Payload{}

	nself, _ := w.api.QueryRead(w.api.Self().String())

	srv := nself.GetService("websocket")

	config, _ := ptypes.MarshalAny(w.cfg)
	srv.Config = config

	w.api.QueryUpdate(nself)
}

func (w *WebSocket) NewConfig() proto.Message {
	const (
		writeWait      = 10 * time.Second    // Time allowed to write a message to the peer.
		pongWait       = 60 * time.Second    // Time allowed to read the next pong message from the peer.
		pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period. Must be less than pongWait.
		maxMessageSize = 512                 // Maximum message size allowed from peer.
	)
	return &pb.WebSocketConfig{
		Port:           3142,
		Tick:           "5ms",
		WriteWait:      writeWait.String(),
		PongWait:       pongWait.String(),
		PingPeriod:     pingPeriod.String(),
		MaxMessageSize: maxMessageSize,
	}
}

func (w *WebSocket) ConfigURL() string {
	a, _ := ptypes.MarshalAny(w.NewConfig())
	return a.GetTypeUrl()
}

func (w *WebSocket) setupRouter() {
	w.router = mux.NewRouter()
	w.router.HandleFunc("/ws", func(wr http.ResponseWriter, req *http.Request) {
		w.serveWs(w.hub, wr, req)
	})
}

func (w *WebSocket) startServer() {
	url := &url.URL{
		Host: net.JoinHostPort(w.srvIp.String(), strconv.Itoa(int(w.cfg.Port))),
	}
	if url.Hostname() == "" || url.Port() == "" {
		w.api.Logf(lib.LLERROR, "Hostname or Port is empty! Hostname: %v Port: %v Host: %v", url.Hostname(), url.Port(), url.Host)
		return
	}

	for {
		w.srv = &http.Server{
			Handler: handlers.CORS(
				handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
				handlers.AllowedOrigins([]string{"*"}),
				handlers.AllowedMethods([]string{"PUT", "GET", "POST", "DELETE"}),
			)(w.router),
			Addr:         url.Host,
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}
		w.api.Logf(lib.LLINFO, "websocket is listening on: %s\n", w.srv.Addr)
		if e := w.srv.ListenAndServe(); e != nil {
			if e != http.ErrServerClosed {
				w.api.Logf(lib.LLNOTICE, "http stopped: %v\n", e)
			}
		}
		w.api.Log(lib.LLNOTICE, "websocket listener stopped")

	}
}

func (w *WebSocket) srvStop() {
	w.api.Log(lib.LLDEBUG, "websocket is shutting down listener")
	w.srv.Shutdown(context.Background())
}

func init() {
	module := &WebSocket{}
	si := core.NewServiceInstance(
		"websocket",
		module.Name(),
		module.Entry,
	)

	discovers := make(map[string]map[string]reflect.Value)
	discovers[WsStateURL] = map[string]reflect.Value{
		"RUN": reflect.ValueOf(cpb.ServiceInstance_RUN)}

	core.Registry.RegisterModule(module)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

func (w *WebSocket) newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []*Payload),
		action:     make(chan *Action),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		api:        w.api,
	}
}

func (h *Hub) run() {
	h.api.Logf(lib.LLDDDEBUG, "Starting Hub\n")
	for {
		select {
		case client := <-h.register:
			h.api.Logf(lib.LLDDDEBUG, "hub registering new client %p", client)
			h.clients[client] = true
			h.api.Logf(lib.LLDDDEBUG, "hub client list: %+v", h.clients)
		case client := <-h.unregister:
			h.api.Logf(lib.LLDDDEBUG, "hub unregistering client %p", client)
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.api.Logf(lib.LLDDDEBUG, "hub client list: %+v", h.clients)
		case messages := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- messages:
				default:
					h.api.Logf(lib.LLERROR, "Websocket channel buffer overflow. closing websocket. messages not sent: %v", messages)
					close(client.send)
					delete(h.clients, client)
				}
			}
		case action := <-h.action:
			switch action.Command {
			case SUBSCRIBE:
				h.api.Logf(lib.LLDDDEBUG, "Subscribing to: %v", action.EventType)
				action.Client.subscribeEvent(lib.EventTypeValue[action.EventType])
			case UNSUBSCRIBE:
				h.api.Logf(lib.LLDDDEBUG, "Unsubscribing from: %v", action.EventType)
				action.Client.unsubscribeEvent(lib.EventTypeValue[action.EventType])
			default:
				h.api.Logf(lib.LLDDEBUG, "Hub received action that has unknown command: %v", action.Command)
			}
		}
	}
}

// write sends messages from the hub to the websocket connection.
//
// A goroutine running write is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) write() {
	pingPeriod, err := time.ParseDuration(c.w.cfg.PingPeriod)
	if err != nil {
		c.w.api.Logf(lib.LLERROR, "%v", err)
	}
	writeWait, err := time.ParseDuration(c.w.cfg.WriteWait)
	if err != nil {
		c.w.api.Logf(lib.LLERROR, "%v", err)
	}
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case messages, ok := <-c.send:
			c.w.api.Logf(lib.LLDDDEBUG, "client %p got messages from hub: %+v", c, messages)
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.w.api.Logf(lib.LLDDEBUG, "hub closed the channel")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			var finalMessages []*Payload
			for _, m := range messages {
				if _, ok := c.subs[m.Type]; ok {
					finalMessages = append(finalMessages, m)
				}
			}

			c.w.api.Logf(lib.LLDDDEBUG, "sending messages to client: %v\n", finalMessages)
			if len(finalMessages) != 0 {
				err := c.conn.WriteJSON(finalMessages)
				if err != nil {
					c.w.api.Logf(lib.LLERROR, "Error writing json to websocket connection%v\n", err)
				}
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// read checks for close messages from the websocket connection.
//
// The application runs read in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) read() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	pongWait, err := time.ParseDuration(c.w.cfg.PongWait)
	if err != nil {
		c.w.api.Logf(lib.LLERROR, "%v", err)
	}
	c.conn.SetReadLimit(c.w.cfg.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.w.api.Logf(lib.LLERROR, "websocket received unexpected close error: %v", err)
			} else {
				c.w.api.Logf(lib.LLERROR, "websocket read error: %v", err)
			}
			break
		}
		action := &Action{
			Client: c,
		}
		json.Unmarshal(message, &action)
		c.w.api.Logf(lib.LLDDDEBUG, "client got message from websocket connection: %v", action)
		c.hub.action <- action
	}
	c.w.api.Logf(lib.LLDDEBUG, "Closing websocket client: %p\n", c)
}

func (c *Client) subscribeEvent(t lib.EventType) {
	c.subs[t] = true
}

func (c *Client) unsubscribeEvent(t lib.EventType) {
	delete(c.subs, t)
}

func (w *WebSocket) serveWs(hub *Hub, wrt http.ResponseWriter, req *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(wrt, req, nil)
	if err != nil {
		w.api.Logf(lib.LLERROR, "Error upgrading websocket connection: %v", err)
		return
	}
	// Creating client with buffered payload channel set to 50. This might have to be increased if we have a lot of nodes
	client := &Client{hub: hub, conn: conn, send: make(chan []*Payload, 50), w: w, subs: make(map[lib.EventType]bool)}
	w.api.Logf(lib.LLDDDEBUG, "websocket added new client: %p\n", client)
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.write()
	go client.read()
}
