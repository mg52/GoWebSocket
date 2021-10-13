package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

func serveWs(id int, pool *Pool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("WebSocket Endpoint Hit")
	conn, err := Upgrade(w, r)
	if err != nil {
		fmt.Fprintf(w, "%+v\n", err)
	}

	//stringId := string(id)
	//fmt.Println("Id: " + fmt.Sprintf())
	client := &Client{
		ID:   strconv.Itoa(id),
		Conn: conn,
		Pool: pool,
	}

	pool.Register <- client
	client.Read()
}

func setupRoutes() {
	pool := NewPool()
	go pool.Start()

	currentId := 0
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		currentId = currentId + 1
		serveWs(currentId, pool, w, r)
	})
}

func main() {
	fmt.Println("Distributed Chat App v0.01")
	setupRoutes()
	http.ListenAndServe(":8080", nil)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return ws, err
	}
	return ws, nil
}

type Pool struct {
	Register   chan *Client
	Unregister chan *Client
	Clients    map[*Client]bool
	Broadcast  chan Message
}

func NewPool() *Pool {
	return &Pool{
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan Message),
	}
}

type Client struct {
	ID   string
	Conn *websocket.Conn
	Pool *Pool
}

type Message struct {
	Type int    `json:"type"`
	Body string `json:"body"`
}

func (c *Client) Read() {
	defer func() {
		c.Pool.Unregister <- c
		c.Conn.Close()
	}()

	for {
		messageType, p, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		message := Message{Type: messageType, Body: string(p)}
		//c.Pool.Broadcast <- message
		fmt.Printf("Message Received: %+v\n", message)
		if err := c.Conn.WriteMessage(messageType, []byte("Hello single client")); err != nil {
			log.Println(err)
			return
		}
	}
}

func (pool *Pool) Start() {
	for {
		select {
		case currentClient := <-pool.Register:
			pool.Clients[currentClient] = true
			fmt.Println("Size of Connection Pool: ", len(pool.Clients))
			for client, _ := range pool.Clients {
				if currentClient.ID != client.ID {
					client.Conn.WriteJSON(Message{Type: 1, Body: "User " + currentClient.ID + " joined. Total user: " + fmt.Sprint(len(pool.Clients))})
				}
			}
		case currentClient := <-pool.Unregister:
			delete(pool.Clients, currentClient)
			fmt.Println("Size of Connection Pool: ", len(pool.Clients))
			for client, _ := range pool.Clients {
				if currentClient.ID != client.ID {
					client.Conn.WriteJSON(Message{Type: 1, Body: "User " + currentClient.ID + " left. Total user: " + fmt.Sprint(len(pool.Clients))})
				}
			}
		case message := <-pool.Broadcast:
			fmt.Println("Sending message to all clients in Pool")
			for client, _ := range pool.Clients {
				if err := client.Conn.WriteJSON(message); err != nil {
					fmt.Println(err)
					return
				}
			}
		}
	}
}
