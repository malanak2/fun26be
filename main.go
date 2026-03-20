package main

import (
	"image/color"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	lobbies := make(map[int]*Lobby)

	r := mux.NewRouter()
	r.HandleFunc("/join/{lobby}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		vars := mux.Vars(r)
		q := r.URL.Query()
		if q.Get("name") == "" {
			http.Error(w, "Missing required parameters", http.StatusBadRequest)
			return
		}
		i, err := strconv.Atoi(vars["lobby"])
		if err != nil {
			http.Error(w, "lobby.invalid", http.StatusBadRequest)
			return
		}
		if lobbies[i] != nil {
			if lobbies[i].Password != q.Get("password") {
				http.Error(w, "password.invalid", http.StatusBadRequest)
				return
			}
			ws, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				http.Error(w, "lobby.invalid", http.StatusBadRequest)
				return
			}
			pl := &Player{Ws: ws, Name: q.Get("name"), L: lobbies[i]}
			lobbies[i].JoinTeam(pl, 0)
			go pl.ReceiveLoop()
			pl.L.Broadcast(NewPacketMessage("player.join", []string{pl.Name}))
			//			pl.SendPacket(lobbies[i])
		}
	})
	r.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		q := r.URL.Query()
		if q.Get("name") == "" || q.Get("lname") == "" /* || q.Get("lcolor") == ""*/ {
			http.Error(w, "Missing required parameters", http.StatusBadRequest)
			return
		}
		limit, err := strconv.Atoi(q.Get("limit"))
		if err != nil {
			limit = 10
		}
		/*col, err := colorx.ParseHexColor(q.Get("lcolor"))
		if err != nil {
			http.Error(w, "Invalid color", http.StatusBadRequest)
			return
		}*/
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal("test" + err.Error())
			return
		}

		pl := &Player{Ws: ws, Name: q.Get("name"), L: nil}

		// TODO: Overlapping lobbies
		lid := rand.Intn(899999) + 100000
		_, exists := lobbies[lid]
		rec := 0
		for exists {
			rec++
			if rec > 11 {
				http.Error(w, "Cannot create more lobbies", http.StatusInsufficientStorage)
				return
			}
			lid = rand.Intn(899999) + 100000
			_, exists = lobbies[lid]
		}
		lobbies[lid] = CreateLobby(pl, q.Get("lname"), limit, color.RGBA{}, q.Get("password"))
		pl.L = lobbies[lid]
		go pl.ReceiveLoop()
		pl.Ws.WriteJSON("{id:" + strconv.Itoa(lid) + "}")
	})
	//	http.Handle("/", r)
	r.Use(mux.CORSMethodMiddleware(r))
	log.Fatal(http.ListenAndServe(":8080", r))
}
