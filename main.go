package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

var pool0 *Pool

func serveWs(id int, pool *Pool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("WebSocket Endpoint Hit")
	conn, err := Upgrade(w, r)
	if err != nil {
		fmt.Fprintf(w, "%+v\n", err)
	}

	client := &Client{
		ID:   strconv.Itoa(id),
		Conn: conn,
		Pool: pool,
	}

	pool.Register <- client
	client.Read()
}

func setupRoutes() {
	currentId := -1
	pool0 = NewPool(0)
	go pool0.Start()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		currentId = currentId + 1
		serveWs(currentId, pool0, w, r)
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
	Id         int
	Register   chan *Client
	Unregister chan *Client
	Clients    map[*Client]bool
	Broadcast  chan Message
}

func NewPool(id int) *Pool {
	return &Pool{
		Id:         id,
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
		fmt.Printf("Message \"%v\" received from User %s\n", string(p), c.ID)
		message := Message{Type: messageType, Body: fmt.Sprintf("User %s says %v", c.ID, string(p))}

		c.Pool.Broadcast <- message
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
					// send message to other users except the new user
					client.Conn.WriteJSON(Message{Type: 1, Body: "User " + currentClient.ID + " joined. Total user: " + fmt.Sprint(len(pool.Clients))})
				} else {
					// send message to the new user
					client.Conn.WriteJSON(Message{Type: 1, Body: "Welcome User " + currentClient.ID + ". Total user: " + fmt.Sprint(len(pool.Clients))})
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
