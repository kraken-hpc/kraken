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
	"os"
	"sync"
	"time"

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

type Action struct {
	Command   Command       `json:"command"`
	EventType lib.EventType `json:"type"`
	Client    *Client
}

func (w *WebSocket) Entry() {
	fmt.Println("websocket entry")
	nself, _ := w.api.QueryRead(w.api.Self().String())
	fmt.Printf("nself: %+v\n", nself.GetServiceIDs())
	s := nself.GetService("restapi")
	c := s.Config()
	fmt.Printf("Value: %v \n", string(c.GetValue()))
	v, e := nself.GetValue(w.cfg.GetAddrUrl())
	fmt.Printf("newvalue: %v error: %v\n", v, e)
	fmt.Printf("id: %+v\n", s.ID())
	fmt.Printf("state: %+v\n", s.State())
	fmt.Printf("getstate: %+v\n", s.GetState())
	fmt.Printf("module: %+v\n", s.Module())
	fmt.Printf("exe: %+v\n", s.Exe())
	fmt.Printf("config: %+v\n", s.Config())
	fmt.Printf("message: %+v\n", s.Message())
	panic("something")
	// w.api.Logf(lib.LLDEBUG, "queried for self: %+v", v)
	// w.srvIp = IPv4.BytesToIP(v.Bytes())
	w.setupRouter()
	go w.startServer()
	w.hub = w.newHub()
	go w.hub.run()
	w.api.Logf(lib.LLDEBUG, "WebSocket is listening for all events\n")
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
		// w.api.Logf(lib.LLDEBUG, "got mutation event: %+v\n", ev.Data().(*core.MutationEvent).String())
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.MutationEvent).String(),
			NodeId: nodeID,
			Value:  ev.Data().(*core.MutationEvent).Mutation[1],
		}
	case lib.Event_STATE_CHANGE:
		// w.api.Logf(lib.LLDEBUG, "got state change event: %+v\n", ev.Data().(*core.StateChangeEvent).Value.Interface())
		payload = &Payload{
			Type:   ev.Type(),
			URL:    url,
			Data:   ev.Data().(*core.StateChangeEvent).String(),
			NodeId: nodeID,
			Value:  lib.ValueToString(ev.Data().(*core.StateChangeEvent).Value),
		}
	case lib.Event_DISCOVERY:
		// w.api.Logf(lib.LLDEBUG, "got discovery event: %+v\n", ev.Data().(*core.DiscoveryEvent).String())
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

func (w *WebSocket) UpdateConfig(cfg proto.Message) (e error) {
	if wc, ok := cfg.(*pb.WebSocketConfig); ok {
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
	if w.cfg == nil {
		w.cfg = w.NewConfig().(*pb.WebSocketConfig)
	}
	w.mutex = &sync.Mutex{}
	w.queue = []*Payload{}
}

func (w *WebSocket) NewConfig() proto.Message {
	const (
		writeWait      = 10 * time.Second    // Time allowed to write a message to the peer.
		pongWait       = 60 * time.Second    // Time allowed to read the next pong message from the peer.
		pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period. Must be less than pongWait.
		maxMessageSize = 512                 // Maximum message size allowed from peer.
	)
	return &pb.WebSocketConfig{
		AddrUrl:        "type.googleapis.com/proto.RestAPIConfig/addr",
		Port:           3142,
		Tick:           "1ms",
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
	for {
		w.srv = &http.Server{
			Handler: handlers.CORS(
				handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}),
				handlers.AllowedOrigins([]string{"*"}),
				handlers.AllowedMethods([]string{"PUT", "GET", "POST", "DELETE"}),
			)(w.router),
			Addr:         fmt.Sprintf("%s:%d", w.srvIp, w.cfg.Port),
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
	core.Registry.RegisterModule(module)
	si := core.NewServiceInstance(
		"websocket",
		module.Name(),
		module.Entry,
		nil,
	)
	core.Registry.RegisterServiceInstance(module, map[string]lib.ServiceInstance{
		si.ID(): si,
	})
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
	h.api.Logf(lib.LLDEBUG, "Starting Hub\n")
	for {
		select {
		case client := <-h.register:
			h.api.Logf(lib.LLDEBUG, "hub registering new client %p", client)
			h.clients[client] = true
			h.api.Logf(lib.LLDDDEBUG, "hub client list: %+v", h.clients)
		case client := <-h.unregister:
			h.api.Logf(lib.LLDEBUG, "hub unregistering client %p", client)
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.api.Logf(lib.LLDEBUG, "hub client list: %+v", h.clients)
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
				h.api.Logf(lib.LLDEBUG, "Subscribing to: %v", lib.EventTypeString[action.EventType])
				action.Client.subscribeEvent(action.EventType)
			case UNSUBSCRIBE:
				h.api.Logf(lib.LLDEBUG, "Unsubscribing to: %v", lib.EventTypeString[action.EventType])
				action.Client.unsubscribeEvent(action.EventType)
			default:
				h.api.Logf(lib.LLDEBUG, "received action has unknown command: %v", action.Command)
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
			c.w.api.Logf(lib.LLDDEBUG, "client %p got messages from hub: %+v", c, messages)
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

			c.w.api.Logf(lib.LLDEBUG, "sending messages to client: %v\n", finalMessages)
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
			}
			c.w.api.Logf(lib.LLERROR, "websocket read error: %v", err)

			break
		}
		c.w.api.Logf(lib.LLDEBUG, "client got message: %v", message)
		action := &Action{
			Client: c,
		}
		json.Unmarshal(message, &action)
		c.hub.action <- action
	}
	c.w.api.Logf(lib.LLDEBUG, "Closing websocket client: %p\n", c)
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
	w.api.Logf(lib.LLDEBUG, "websocket added new client: %p\n", client)
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.write()
	go client.read()
}
