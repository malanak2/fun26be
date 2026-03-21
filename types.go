package main

// TODO: Localizace pro frontend
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icza/gox/imagex/colorx"
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
		p.L.Broadcast(NewPacketString("playerLeave", "player.leave", []string{p.Name, p.L.Teams[0].Name}))
		p.L.KickPlayer(p, "player.left", []string{p.Name})
		if p == p.L.Owner {
			p.L.DestroyLobby()
		}
	}()
	for {
		_, message, err := p.Ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("Connection closed abruptly", "err", err.Error())
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		slog.Info("Received packet from player", "player", p.Name, "packet", string(message))
		p.HandlePacket(string(message))
	}
}

type PacketAny struct {
	Packet
	Message string
	Args    []any
}

func NewPacketAny(mtype string, message string, args []any) PacketAny {
	return PacketAny{Packet{mtype}, message, args}
}

func (p *PacketAny) Compile() PacketMessage {
	r := PacketMessage{Packet: p.Packet, Message: p.Message, Args: make([]string, 0)}
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
			if p.L.IsPlayerAdmin(p.L.FindPlayer(msg.Args[0])) {
				p.SendPacket(NewPacketString("playerKick", "error.playerIsAdmin", []string{}))
				return
			}
			err := p.L.KickPlayerByName(msg.Args[0], "kick.byAnotherPlayer", []string{p.Name})
			if err != nil {
				p.SendPacket(NewPacketString("playerKick", "error.noSuchPlayer", []string{}))
				return
			}
			p.SendPacket(NewPacketString("playerKick", "kick.success", []string{}))
			return
		}
		if msg.Mtype == "promote" {
			player := p.L.FindPlayer(msg.Args[0])
			if player == nil {
				p.SendPacket(NewPacketString("promotePlayer", "error.noSuchPlayer", []string{msg.Args[0]}))
				return
			}
			if slices.Contains(p.L.Admins, player) {
				p.SendPacket(NewPacketString("promotePlayer", "error.playerIsAdmin", []string{msg.Args[0]}))
			}
			p.L.Admins = append(p.L.Admins, p.L.FindPlayer(msg.Args[0]))
			player.SendPacket(NewPacketString("promoted", "promote.promotedBy", []string{p.Name}))
			p.SendPacket(NewPacketString("promotePlayer", "promote.success", []string{msg.Args[0]}))
			return
		}
		if msg.Mtype == "demote" {
			player := p.L.FindPlayer(msg.Args[0])
			if player == nil {
				p.SendPacket(NewPacketString("demotePlayer", "error.noSuchPlayer", []string{msg.Args[0]}))
				return
			}
			if slices.Contains(p.L.Admins, player) {
				i := slices.Index(p.L.Admins, player)
				if i > 0 {
					if i < len(p.L.Admins)-1 {
						p.L.Admins = append(p.L.Admins[:i], p.L.Admins[i+1:]...)
						player.SendPacket(NewPacketString("demoted", "demoted.demotedBy", []string{p.Name}))
						p.SendPacket(NewPacketString("demotePlayer", "demote.success", []string{msg.Args[0]}))
						return
					}
					p.L.Admins = p.L.Admins[:i]
					player.SendPacket(NewPacketString("demoted", "demoted.demotedBy", []string{p.Name}))
					p.SendPacket(NewPacketString("demotePlayer", "demote.success", []string{msg.Args[0]}))
					return
				}
				p.L.Admins = p.L.Admins[i:]
				player.SendPacket(NewPacketString("demoted", "demoted.demotedBy", []string{p.Name}))
				p.SendPacket(NewPacketString("demotePlayer", "demote.success", []string{msg.Args[0]}))
				return
			}
			p.SendPacket(NewPacketString("demotePlayer", "error.playerNotAdmin", []string{msg.Args[0]}))
			return
		}
		if msg.Mtype == "createTeam" {
			col, err := colorx.ParseHexColor(msg.Args[1])
			if err != nil {
				p.SendPacket(NewPacketError("createTeam.failed", []string{"Bad color"}))
				slog.Warn("Failed to create team - bad color", "error", err.Error())
				return
			}
			t := Team{
				Players: make([]*Player, 0),
				Name:    msg.Args[0],
				Color:   col,
			}
			p.L.Teams = append(p.L.Teams, &t)
			p.L.Broadcast(NewPacketAny("newTeam", strconv.Itoa(len(p.L.Teams)-1), []any{msg.Args[0], col}))
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
		ret := make([]string, 0)
		i, err := strconv.Atoi(msg.Args[0])
		if err != nil || i >= len(p.L.Teams) {
			p.SendPacket(NewPacketError("error.badTeam", []string{}))
		}
		for _, pp := range p.L.Teams[i].Players {
			ret = append(ret, pp.Name)
		}
		p.SendPacket(PacketMessage{Packet: Packet{Mtype: "contestants"}, Message: strconv.Itoa(i), Args: ret})
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
			strconv.Itoa(i),
			[]any{NWTeamFromTeam(p.L.Teams[i])}))
		return
	}
	if msg.Mtype == "moveTeam" {
		i, err := strconv.Atoi(msg.Args[0])
		if err != nil || len(p.L.Teams) <= i {
			p.SendPacket(NewPacketError("changeTeam.badId", []string{}))
			return
		}
		p.L.RemovePlayer(p)
		p.L.JoinTeam(p, i)
		p.L.Broadcast(NewPacketInt("changeTeam", p.Name, []int{i}))
		return
	}
	p.SendPacket(NewPacketError("error.badPacket", []string{}))
}

type Team struct {
	Players []*Player
	Name    string
	Color   color.RGBA
	Score   int
}

type NWTeam struct {
	Name  string
	Color color.RGBA
	Score int
}

func NWTeamFromTeam(t *Team) NWTeam {
	return NWTeam{Name: t.Name, Color: t.Color, Score: t.Score}
}

type Lobby struct {
	Limit    int
	Teams    []*Team
	Admins   []*Player
	Owner    *Player
	HasBegun bool
	Password string
}

func (l *Lobby) IsPlayerAdmin(p *Player) bool {
	return slices.Contains(l.Admins, p)
}

func CreateLobby(creator *Player, name string, limit int, clr color.RGBA, password string) *Lobby {
	slog.Info("Creating lobby", "owner", creator.Name, "lName", name)
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
		Owner:    creator,
		HasBegun: false,
		Password: password,
	}
}

func (l *Lobby) AddTeam(name string, clr color.RGBA) int {
	slog.Info("Creating team", "name", name)
	l.Teams = append(l.Teams, &Team{Name: name, Color: clr})
	return len(l.Teams) - 1
}

func (l *Lobby) JoinTeam(pl *Player, team int) {
	slog.Info("Player joining team", "player", pl.Name, "team", team)
	l.Teams[team].Players = append(l.Teams[team].Players, pl)
}

func (l *Lobby) RemovePlayer(pl *Player) {
	for i, t := range l.Teams {
		if slices.Contains(t.Players, pl) {
			slog.Info("Removing player from team", "player", pl.Name, "team", i)
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
			slog.Info("Sending message to player", "msg", message, "player", p.Name)
			p.SendPacket(message)
		}
	}
}

func (l *Lobby) BroadcastMessage(message string, args []string) {
	slog.Info("Broadcasting a mesages", "msg", message)
	l.Broadcast(NewPacketMessage(message, args))
}

func (l *Lobby) Leave(pl *Player) {
	l.RemovePlayer(pl)
	l.BroadcastMessage("player.left", []string{pl.Name})
	if pl == l.Owner {
		l.DestroyLobby()
	}
}

func (l *Lobby) ChangeTeam(pl *Player, team int) {
	slog.Info("Player changing teams", "player", pl.Name, "newTeamId", team)
	l.RemovePlayer(pl)
	l.JoinTeam(pl, team)
}

func (l *Lobby) KickPlayer(pl *Player, reason string, args []string) {
	slog.Info("Kicking player", "player", pl.Name, "reason", reason)
	l.RemovePlayer(pl)
	pl.SendPacket(NewPacketDisconnect(reason, args))
	pl.Ws.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(1))
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
	slog.Info("Destroying lobby")
	for _, t := range l.Teams {
		for _, p := range t.Players {
			l.KickPlayer(p, "lobby.close", []string{})
		}
	}
	l.Admins = nil
	l.Teams = nil
}
