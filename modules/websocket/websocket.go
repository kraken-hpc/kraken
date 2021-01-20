/* websocket.go: this module provides websocket capabilities for the restfulapi
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugins=grpc:. websocket.proto

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
	"time"

	cpb "github.com/hpc/kraken/core/proto"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
	"github.com/hpc/kraken/lib/util"
)

var _ types.Module = (*WebSocket)(nil)
var _ types.ModuleSelfService = (*WebSocket)(nil)
var _ types.ModuleWithConfig = (*WebSocket)(nil)
var _ types.ModuleWithAllEvents = (*WebSocket)(nil)
var _ types.ModuleWithDiscovery = (*WebSocket)(nil)

const WsStateURL = "/Services/websocket/State"

type Command uint8

const (
	SUBSCRIBE Command = iota
	UNSUBSCRIBE
)

type WebSocket struct {
	cfg    *Config
	api    types.ModuleAPIClient
	router *mux.Router
	srv    *http.Server
	echan  <-chan types.Event
	dchan  chan<- types.Event
	hub    *Hub
	ticker *time.Ticker
	srvIp  net.IP
}

type Hub struct {
	clients    map[*Client]bool // Registered clients.
	broadcast  chan *Payload    // Messages from the event stream.
	action     chan *Action     // Messages from the websocket Clients.
	register   chan *Client     // Register requests from the clients.
	unregister chan *Client     // Unregister requests from clients.
	api        types.ModuleAPIClient
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn          // The websocket connection.
	send chan *Payload            // Buffered channel of outbound messages.
	w    *WebSocket               // Web Socket
	subs map[types.EventType]bool // List of event types that client is subscribed to
}

type Payload struct {
	Type   types.EventType `json:"type"`
	URL    string          `json:"url"`
	Data   string          `json:"data"`
	NodeId string          `json:"nodeid"`
	Value  string          `json:"value"`
}

func (p *Payload) String() string {
	return fmt.Sprintf("Type: %v, Url: %v, Data: %v, NodeId: %v, Value: %v", types.EventTypeString[p.Type], p.URL, p.Data, p.NodeId, p.Value)
}

type Action struct {
	Command   Command `json:"command"`
	EventType string  `json:"type"`
	Client    *Client
}

func (w *WebSocket) Entry() {
	nself, _ := w.api.QueryRead(w.api.Self().String())
	// Update config so restapi knowns which port to use
	srv := nself.GetService("websocket")
	config, _ := ptypes.MarshalAny(w.cfg)
	srv.Config = config
	w.api.QueryUpdate(nself)

	rAddr, e := nself.GetValue("/Services/restapi/Config/Addr")
	if e != nil {
		w.api.Logf(types.LLERROR, "error getting restapi address")
	}
	w.srvIp = net.ParseIP(rAddr.String())

	w.setupRouter()
	go w.startServer()
	w.hub = w.newHub()
	go w.hub.run()

	url := util.NodeURLJoin(w.api.Self().String(), WsStateURL)
	ev := core.NewEvent(
		types.Event_DISCOVERY,
		url,
		&core.DiscoveryEvent{
			URL:     url,
			ValueID: "RUN",
		},
	)
	w.dchan <- ev

	w.api.Logf(types.LLDDDEBUG, "starting main loop")
	for {
		select {
		case e := <-w.echan: // event
			go w.handleEvent(e)
			break
		}
	}
}

func (w *WebSocket) handleEvent(ev types.Event) {
	var payload = &Payload{}
	nodeID, url := util.NodeURLSplit(ev.URL())
	switch ev.Type() {
	case types.Event_STATE_MUTATION:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.MutationEvent).String(),
			NodeId: nodeID,
			Value:  ev.Data().(*core.MutationEvent).Mutation[1],
		}
	case types.Event_STATE_CHANGE:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.StateChangeEvent).String(),
			NodeId: nodeID,
			Value:  util.ValueToString(ev.Data().(*core.StateChangeEvent).Value),
		}
	case types.Event_DISCOVERY:
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.DiscoveryEvent).String(),
			NodeId: nodeID,
			Value:  ev.Data().(*core.DiscoveryEvent).ValueID,
		}
	default:
		w.api.Logf(types.LLDEBUG, "got unknown event: %+v\n", ev.Data())
	}
	w.hub.broadcast <- payload
}

func (w *WebSocket) Stop() { os.Exit(0) }

func (w *WebSocket) Name() string { return "github.com/hpc/kraken/modules/websocket" }

// SetEventsChan sets the event channel
// this is generally done by the API
func (w *WebSocket) SetEventsChan(c <-chan types.Event) { w.echan = c }

func (w *WebSocket) SetDiscoveryChan(c chan<- types.Event) { w.dchan = c }

func (w *WebSocket) UpdateConfig(cfg proto.Message) (e error) {
	if wc, ok := cfg.(*Config); ok {
		w.api.Logf(types.LLDEBUG, "updating config for websocket: %v", wc)
		w.cfg = wc
		if w.srv != nil {
			w.srvStop() // we just stop, entry will (re)start
		}
		return
	}
	return fmt.Errorf("wrong config type")
}

func (w *WebSocket) Init(api types.ModuleAPIClient) {
	w.api = api
	w.cfg = w.NewConfig().(*Config)
}

func (w *WebSocket) NewConfig() proto.Message {
	const (
		writeWait      = 10 * time.Second    // Time allowed to write a message to the peer.
		pongWait       = 60 * time.Second    // Time allowed to read the next pong message from the peer.
		pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period. Must be less than pongWait.
		maxMessageSize = 512                 // Maximum message size allowed from peer.
	)
	return &Config{
		Port:           3142,
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
		w.api.Logf(types.LLERROR, "Hostname or Port is empty! Hostname: %v Port: %v Host: %v", url.Hostname(), url.Port(), url.Host)
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
		w.api.Logf(types.LLINFO, "websocket is listening on: %s\n", w.srv.Addr)
		if e := w.srv.ListenAndServe(); e != nil {
			if e != http.ErrServerClosed {
				w.api.Logf(types.LLNOTICE, "http stopped: %v\n", e)
			}
		}
		w.api.Log(types.LLNOTICE, "websocket listener stopped")

	}
}

func (w *WebSocket) srvStop() {
	w.api.Log(types.LLDEBUG, "websocket is shutting down listener")
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
	core.Registry.RegisterServiceInstance(module, map[string]types.ServiceInstance{si.ID(): si})
	core.Registry.RegisterDiscoverable(si, discovers)
}

func (w *WebSocket) newHub() *Hub {
	return &Hub{
		broadcast:  make(chan *Payload),
		action:     make(chan *Action),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		api:        w.api,
	}
}

func (h *Hub) run() {
	h.api.Logf(types.LLDDDEBUG, "Starting Hub\n")
	for {
		select {
		case client := <-h.register:
			h.api.Logf(types.LLDDDEBUG, "hub registering new client %p", client)
			h.clients[client] = true
			h.api.Logf(types.LLDDDEBUG, "hub client list: %+v", h.clients)
		case client := <-h.unregister:
			h.api.Logf(types.LLDDDEBUG, "hub unregistering client %p", client)
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.api.Logf(types.LLDDDEBUG, "hub client list: %+v", h.clients)
		case messages := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- messages:
				default:
					h.api.Logf(types.LLERROR, "Websocket channel buffer overflow. closing websocket. messages not sent: %v", messages)
					close(client.send)
					delete(h.clients, client)
				}
			}
		case action := <-h.action:
			switch action.Command {
			case SUBSCRIBE:
				h.api.Logf(types.LLDDDEBUG, "Subscribing to: %v", action.EventType)
				action.Client.subscribeEvent(types.EventTypeValue[action.EventType])
			case UNSUBSCRIBE:
				h.api.Logf(types.LLDDDEBUG, "Unsubscribing from: %v", action.EventType)
				action.Client.unsubscribeEvent(types.EventTypeValue[action.EventType])
			default:
				h.api.Logf(types.LLDDEBUG, "Hub received action that has unknown command: %v", action.Command)
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
		c.w.api.Logf(types.LLERROR, "%v", err)
	}
	writeWait, err := time.ParseDuration(c.w.cfg.WriteWait)
	if err != nil {
		c.w.api.Logf(types.LLERROR, "%v", err)
	}
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.w.api.Logf(types.LLDDDEBUG, "client %p got message from hub: %+v", c, message)
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.w.api.Logf(types.LLDDEBUG, "hub closed the channel")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if _, ok := c.subs[message.Type]; ok {
				c.w.api.Logf(types.LLDDDEBUG, "sending message to client: %v\n", message)
				err := c.conn.WriteJSON(message)
				if err != nil {
					c.w.api.Logf(types.LLERROR, "Error writing json to websocket connection%v\n", err)
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
		c.w.api.Logf(types.LLERROR, "%v", err)
	}
	c.conn.SetReadLimit(c.w.cfg.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.w.api.Logf(types.LLERROR, "websocket received unexpected close error: %v", err)
			} else {
				c.w.api.Logf(types.LLERROR, "websocket read error: %v", err)
			}
			break
		}
		action := &Action{
			Client: c,
		}
		json.Unmarshal(message, &action)
		c.w.api.Logf(types.LLDDDEBUG, "client got message from websocket connection: %v", action)
		c.hub.action <- action
	}
	c.w.api.Logf(types.LLDDEBUG, "Closing websocket client: %p\n", c)
}

func (c *Client) subscribeEvent(t types.EventType) {
	c.subs[t] = true
}

func (c *Client) unsubscribeEvent(t types.EventType) {
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
		w.api.Logf(types.LLERROR, "Error upgrading websocket connection: %v", err)
		return
	}
	// Creating client with buffered payload channel set to 50. This might have to be increased if we have a lot of nodes
	client := &Client{hub: hub, conn: conn, send: make(chan *Payload, 50), w: w, subs: make(map[types.EventType]bool)}
	w.api.Logf(types.LLDDDEBUG, "websocket added new client: %p\n", client)
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.write()
	go client.read()
}
