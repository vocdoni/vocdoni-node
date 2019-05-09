package ws

import (
	"fmt"
	"net/http"
	"encoding/json"
	"log"
	
    "github.com/gorilla/mux"
    "github.com/gorilla/websockets"

	"github.com/vocdoni/go-dvote/types"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan *types.Message)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true	
	},
}

func initRouter() {
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler).Methods("POST")
	router.HandleFunc("/ws", wsHandler)

	go echo()
	log.Fatal(http.ListenAndServe(":8445", router))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if err := json.NewDecoder(r.Body).Decode(&method); err != nil {
		log.Printf("ERROR: %s", err)
		http.Error(w, "Bad request", http.StatusTeapot)
		return
	}
	defer r.Body.Close()
	go writer(&method)
}

func writer(method string) {
	broadcast <- msg
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
			log.Fatal(err)
	}

	// register client
	clients[ws] = true	
}

func echo() {
	for {
			val := <-broadcast
			method := fmt.Sprintf("%s", method)
			// send to every client that is currently connected
			for client := range clients {
					err := client.WriteMessage(websocket.TextMessage, []byte(method))
					if err != nil {
							log.Printf("Websocket error: %s", err)
							client.Close()
							delete(clients, client)
					}
			}
	}
}

