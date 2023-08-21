package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type ClientManger struct {
	// The client map stores and manages all long connection clients, online is True,
	// and those who are not there are FALSE
	clients map[*Client]bool
	// Web sude MESSAGE we use broadcast to recieve, and finally distribute
	// it to all clients.
	broadcast chan []byte
	// Newly created long connection client
	register chan *Client
	// Newly canceled long conneciton client
	unregister chan *Client
}

type Client struct {
	id string
	// connected socket
	socket *websocket.Conn
	// Message
	send chan []byte
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"Recipient,omitempty"`
	Content   string `json:"Content,omitempty"`
	ServerIP  string `json:"serverIp,omitempty"`
	SenderIP  string `json:"senderIp,omitempty"`
}

var manager = ClientManger{
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

func (manager *ClientManger) start() {
	for {
		select {
		// if there is a new connection access, pass the connection to
		// con through the channel
		case conn := <-manager.register:
			// set the client connection to true
			manager.clients[conn] = true
			// format the message of returning to successful connection JSON
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected. ",
				ServerIP: LocalIp(), SenderIP: conn.socket.RemoteAddr().String()})
			// Call the client's send method and send messages
			manager.send(jsonMessage, conn)
			// if the connection is disconneted
		case conn := <-manager.unregister:
			// determine the state of the connection, it it is true, turn off Send and deleted the value of connecting client
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected. ",
					ServerIP: LocalIp(), SenderIP: conn.socket.RemoteAddr().String()})
				manager.send(jsonMessage, conn)
			}
		// broadcast
		case message := <-manager.broadcast:
			// traversing the clinet that has been connected, send the message to them
			for conn := range manager.clients {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(manager.clients, conn)
				}
			}
		}
	}
}

func (manager *ClientManger) send(message []byte, ignore *Client) {
	for conn := range manager.clients {
		// send messages not to the shielded connection
		if conn != ignore {
			conn.send <- message
		}
	}
}

// Define the read method of the client structure
func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		_ = c.socket.Close()
	}()

	for {
		// Read message
		_, message, err := c.socket.ReadMessage()
		// it there is an error message, cancel this connection and then close it
		if err != nil {
			manager.unregister <- c
			_ = c.socket.Close()
			break
		}
		// if there is no error message put the information in Broadcast
		jsonMessage, _ := json.Marshal(
			&Message{Sender: c.id, Content: string(message),
				ServerIP: LocalIp(), SenderIP: c.socket.RemoteAddr().String()})
		manager.broadcast <- jsonMessage
	}
}

func (c *Client) write() {
	defer func() {
		_ = c.socket.Close()
	}()

	for {
		select {
		// read the message from send
		case message, ok := <-c.send:
			// if there is no message
			if !ok {
				_ = c.socket.WriteMessage(websocket.TextMessage, message)
			}
		}
	}
}

func main() {
	fmt.Println("Starting application...")
	// Open goroutine execution start program
	go manager.start()
	// Register the defatult route to /ws, and use the wsHandler method
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/health", healthHandler)
	fmt.Println("chat server start.....")
	_ = http.ListenAndServe("0.0.0.0:8448", nil)

}

func wsHandler(res http.ResponseWriter, req *http.Request) {
	// Upgrade the HTTP protocol to the websocket protocol
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if err != nil {
		http.NotFound(res, req)
		return
	}

	client := &Client{
		id: uuid.Must(uuid.NewV4(), nil).String(), socket: conn, send: make(chan []byte)}
	// Register a new link
	manager.register <- client

	// Start the message to collect the news from the web side
	go client.read()
	// Start the corporation to return the message to the web side
	go client.write()
}

func healthHandler(res http.ResponseWriter, _ *http.Request) {
	_, _ = res.Write([]byte("ok"))
}

func LocalIp() string {
	address, _ := net.InterfaceAddrs()
	var ip = "localhost"
	for _, address := range address {
		if ipAddress, ok := address.(*net.IPNet); ok && !ipAddress.IP.IsLoopback() {
			if ipAddress.IP.To4() != nil {
				ip = ipAddress.IP.String()
			}
		}
	}
	return ip
}
