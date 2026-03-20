package main

// TODO: Localizace pro frontend
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"time"

	"github.com/golangci/golangci-lint/v2/pkg/golinters/arangolint"
	"github.com/gorilla/websocket"
)

type Player struct {
	Ws   *websocket.Conn
	L    *Lobby
	Name string
}

type Packet struct {
	Mtype string
}

type PacketMessage struct {
	Packet
	Message string
	Args    []string
}
type PacketInt struct {
	Packet
	Message string
	Args    []int
}

func NewPacketString(mtype string, message string, args []string) PacketMessage {
	return PacketMessage{Packet: Packet{Mtype: mtype}, Message: message, Args: args}
}

func NewPacketMessage(message string, args []string) PacketMessage {
	return PacketMessage{Packet: Packet{Mtype: "message"}, Message: message, Args: args}
}

func NewPacketInt(mtype string, message string, args []int) PacketInt {
	return PacketInt{Packet: Packet{Mtype: mtype}, Message: message, Args: args}
}

type BasePacket struct {
	Packet
	Args []string
}

func NewPacketDisconnect(reason string, args []string) PacketMessage {
	return PacketMessage{Packet: Packet{Mtype: "disconnect"}, Message: reason, Args: args}
}

func (p *Player) SendMessage(text string, args []string) {
	p.SendPacket(NewPacketMessage(text, args))
}

func (p *Player) SendPacket(packet interface{}) {
	p.Ws.WriteJSON(packet)
}

func (p *Player) ReceiveLoop() {
	defer func() {
		p.L.KickPlayer(p, "player.left", []string{p.Name})
	}()
	for {
		_, message, err := p.Ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		p.L.Broadcast(string(message))
		p.HandlePacket(string(message))
	}
}

type PacketAny struct {
	Packet
	message string
	Args    []any
}

func NewPacketAny(mtype string, message string, args []any) PacketAny {
	return PacketAny{Packet{mtype}, message, args}
}

func (p *PacketAny) Compile() PacketMessage {
	r := PacketMessage{Packet: p.Packet, Message: p.message, Args: make([]string, 0)}
	for _, i := range p.Args {
		r.Args = append(r.Args, fmt.Sprintf("%v", i))
	}
	return r
}

func NewPacketError(msg string, args []string) PacketMessage {
	return PacketMessage{Packet: Packet{Mtype: "error"}, Message: msg, Args: args}
}

func (p *Player) HandlePacket(message string) {
	var msg BasePacket
	err := json.Unmarshal([]byte(message), &msg)
	if err != nil {
		p.SendPacket(NewPacketMessage("message.invalidPacket", []string{err.Error()}))
	}
	if slices.Contains(p.L.Admins, p) {
		if msg.Mtype == "kick" {
			p.L.KickPlayerByName(msg.Args[0], "kick.byAnotherPlayer", []string{p.Name})
			return
		}
		if msg.Mtype == "promote" {
			p.L.Admins = append(p.L.Admins, p.L.FindPlayer(msg.Args[0]))
			return
		}
		if msg.Mtype == "demote" {
			if slices.Contains(p.L.Admins, p.L.FindPlayer(msg.Args[0])) {
			}
			return
		}
	}
	if msg.Mtype == "message" {
		p.L.BroadcastMessage(msg.Args[0], []string{p.Name})
		return
	}
	if msg.Mtype == "getTeams" {
		ret := make([]int, 0)
		for i := range p.L.Teams {
			ret = append(ret, i)
		}
		p.SendPacket(NewPacketInt("teams", "", ret))
		return
	}
	if msg.Mtype == "getPeople" {
		ret := make([]string, 1)
		i, err := strconv.Atoi(msg.Args[0])
		if err != nil || i >= len(p.L.Teams) {
			p.SendPacket(NewPacketError("error.badTeam", []string{}))
		}
		for _, pp := range p.L.Teams[i].Players {
			ret = append(ret, pp.Name)
		}
		p.SendPacket(PacketMessage{Packet: Packet{Mtype: "contestants"}, Message: "", Args: ret})
		return
	}
	if msg.Mtype == "getTeam" {
		i, err := strconv.Atoi(msg.Args[0])
		if err != nil || i >= len(p.L.Teams) {
			p.SendPacket(NewPacketError("error.badTeam", []string{}))
			return
		}
		p.SendPacket(NewPacketAny(
			"teamInfo",
			msg.Args[0],
			[]any{p.L.Teams[i]}))
		return
	}
	p.SendPacket(NewPacketError("error.badPacket", []string{}))
}

type Team struct {
	Players []*Player
	Name    string
	Color   color.RGBA
}

type Lobby struct {
	Limit    int
	Teams    []*Team
	Admins   []*Player
	HasBegun bool
	Password string
}

func (l *Lobby) IsPlayerAdmin(p *Player) bool {
	return slices.Contains(l.Admins, p)
}

func CreateLobby(creator *Player, name string, limit int, clr color.RGBA, password string) *Lobby {
	return &Lobby{
		Limit: limit,
		Teams: []*Team{
			{
				Players: []*Player{creator},
				Name:    name,
				Color:   clr,
			},
		},
		Admins:   []*Player{creator},
		HasBegun: false,
		Password: password,
	}
}

func (l *Lobby) AddTeam(name string, clr color.RGBA) int {
	l.Teams = append(l.Teams, &Team{Name: name, Color: clr})
	return len(l.Teams) - 1
}

func (l *Lobby) JoinTeam(pl *Player, team int) {
	l.Teams[team].Players = append(l.Teams[team].Players, pl)
}

func (l *Lobby) RemovePlayer(pl *Player) {
	for _, t := range l.Teams {
		if slices.Contains(t.Players, pl) {
			ind := slices.Index(t.Players, pl)
			t.Players = append(t.Players[:ind], t.Players[ind+1:]...)
		}
	}
}

func (l *Lobby) FindPlayer(name string) *Player {
	for _, t := range l.Teams {
		for _, p := range t.Players {
			if p.Name == name {
				return p
			}
		}
	}
	return nil
}
func (l *Lobby) Broadcast(message interface{}) {
	for _, t := range l.Teams {
		for _, p := range t.Players {
			fmt.Printf("Sending message to player %s %v (%v)\n", p.Name, p.Ws.RemoteAddr(), message)
			p.SendPacket(message)
		}
	}
}

func (l *Lobby) BroadcastMessage(message string, args []string) {
	l.Broadcast(NewPacketMessage(message, args))
}

func (l *Lobby) Leave(pl *Player) {
	l.RemovePlayer(pl)
	l.BroadcastMessage("player.left", []string{pl.Name})
}

func (l *Lobby) ChangeTeam(pl *Player, team int) {
	l.RemovePlayer(pl)
	l.JoinTeam(pl, team)
}

func (l *Lobby) KickPlayer(pl *Player, reason string, args []string) {
	l.RemovePlayer(pl)
	pl.SendPacket(NewPacketDisconnect(reason, args))
	pl.Ws.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(100))
	pl.Ws.Close()
}
func (l *Lobby) KickPlayerByName(name string, reason string, args []string) error {
	pl := l.FindPlayer(name)
	if pl == nil {
		return errors.New("message.playernotfound")
	}
	l.RemovePlayer(pl)
	pl.SendPacket(NewPacketDisconnect(reason, args))
	pl.Ws.Close()
	return nil
}

func (l *Lobby) DestroyLobby() {
	l.Broadcast(NewPacketDisconnect("lobby.close", []string{}))
	l.Admins = nil
	l.Teams = nil
}
