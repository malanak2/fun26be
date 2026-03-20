package main

import (
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/icza/gox/imagex/colorx"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		//        TODO: cors for the domain
		//        origin := r.Header.Get("Origin")
		//        return origin == "https://your.domain.com"
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
		val, ok := lobbies[i]
		if !ok {
			http.Error(w, "lobby.invalid", http.StatusBadRequest)
			return
		}
		if val.Teams == nil {
			http.Error(w, "lobby.invalid", http.StatusBadRequest)
			delete(lobbies, i)
			return
		}
		if val.Password != q.Get("password") {
			http.Error(w, "password.invalid", http.StatusBadRequest)
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "lobby.invalid", http.StatusBadRequest)
			return
		}
		pl := &Player{Ws: ws, Name: q.Get("name"), L: val}
		val.JoinTeam(pl, 0)
		go pl.ReceiveLoop()
		pl.L.Broadcast(NewPacketString("playerJoin", "player.join", []string{pl.Name, "0"}))
	})
	r.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		q := r.URL.Query()
		if q.Get("name") == "" || q.Get("lname") == "" || q.Get("lcolor") == "" {
			http.Error(w, "Missing required parameters", http.StatusBadRequest)
			slog.Warn("Tried to create lobby with invalid params", "origin", r.Header.Get("Origin"), "Params", q)
			return
		}
		// TODO: Implement
		limit, err := strconv.Atoi(q.Get("limit"))
		if err != nil {
			limit = 10
		}
		col, err := colorx.ParseHexColor(q.Get("lcolor"))
		if err != nil {
			http.Error(w, "Invalid color", http.StatusBadRequest)
			slog.Warn("Failed to create lobby - bad color", "error", err.Error())
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("Failed to upgrade", "err", err.Error(), "origin", r.Header.Get("Origin"))
			return
		}

		pl := &Player{Ws: ws, Name: q.Get("name"), L: nil}

		lid := rand.Intn(899999) + 100000
		_, exists := lobbies[lid]
		rec := 0
		for exists {
			rec++
			if rec > 11 {
				slog.Warn("Ran out of lobby numbers")
				http.Error(w, "Cannot create more lobbies", http.StatusInsufficientStorage)
				return
			}
			lid = rand.Intn(899999) + 100000
			_, exists = lobbies[lid]
		}
		lobbies[lid] = CreateLobby(pl, q.Get("lname"), limit, col, q.Get("password"))
		pl.L = lobbies[lid]
		go pl.ReceiveLoop()
		pl.Ws.WriteJSON("{id:" + strconv.Itoa(lid) + "}")
	})
	//	http.Handle("/", r)
	r.Use(mux.CORSMethodMiddleware(r))
	slog.Info("Now serving", "port", "8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
