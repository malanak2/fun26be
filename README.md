## Structure
Packet:
```go
type BasePacket struct {
    Mtype string
    Args []string
}
```
Example request for people of team with id 0:
```json
{
    "Mtype": "getPeople",
    "Args":["0"]
}
``` 


Example return:
```json
{
  "Mtype": "contestants",
  "Message": "0",
  "Args": [
    "Player1",
    "Player2",
  ]
}
```

(in team 0 (Message) there are players Player1 and Player2 (Args))


Simplified notation: getPeople(\[teamId\]) -> contestants(teamId)(\["Player1", "Player2"\])
                    ^ The Mtype ^ Args var ^ resp Mtype ^ Msg  ^Args content

## TO CREATE A LOBBY, THE QUERY PARAMAS THAT ARE REQUIRED ARE name, lname, lat, lon (name = playerName, lname = main team name,lat, lon are beginning coords - use current?)
## TO JOIN A LOBBY, ONLY name IS REQUIRED IN QUERY PARAMS

## Possible packets and their response
### Normal
#### Message
- Sends a message to every player
- message(\["Message to send"\]) -> message("message")(\["Author"\])

#### Get Teams
- This fetches all the team ids
- getTeams() -> teams()(\[0, 1, ...\])

#### Get Team
- This returns the teams name and color
- getTeam(\[teamId\]) -> teamInfo(teamId)(\[{Name: "Name", Color: "NoIdeaTryIt"}\])

#### Get People
- This returns the people in the team
- getPeople(\[teamid\]) -> contestants(teamId)(\["Jan Bures", "Andrej Bures"\])

#### Change Team
- Call this to change team. It broadcasts its response unless it errors out. For that, see Passive events
- moveTeam(\[teamId\]) -> changeTeam(error)(\[\]) 


### Admin packets -- these require the player to be an admin. By default, only the lobby creator is one
#### Kick player
- Kicks a player, returned message is either kick.success, error.noSuchPlayer or error.playerIsAdmin
- kick(\[playerName\]) -> playerKick(Result?)([])
#### Promote a player
- Promotes a non-admin player to admin, Result is either promote.success, error.playerIsAdmin or error.noSuchPlayer.
- promote(\[playerName\]) -> promotePlayer(Result?)([playerName])
- It sends a packet to the promoted player (See Passive events)
#### demote a player
- Demotes an admin player, Result is either demote.success, error.playerNotAdmin or error.noSuchPlayer
- demote(\[playerName\]) -> demotePlayer(Result?)([playerName])
#### Create Team
- Creates a team
- createTeam(\[name, color\]) -> Broadcasted - see in passive events
#### Start Game
- starts the game
- startGame(\[\]) -> Broadcasted - see in passive events

## Passive events - these could happen any time
#### Promoted
- This packet is sent to the promoted player
- promoted(promote.promotedBy)(\[adminWhoPromoted\])
#### Demoted
- This packet is sent to the demoted player
- demoted(demote.demotedBy)(\[adminWhoDemoted\])
#### Message
- This event comes in in case of a message
- message(msgText)(\[authorName\])
#### Disconnect
- This usually comes either when the lobby closes or the player is kicked - either way it signals the termination of the connection
- disconnect(Message)(\[byWhoName\])
#### Player Joined
- When a player joins, this is sent to everyone
- playerJoin(player.join)(\[playerName, teamId\])
#### Player Left
- When a player leaves the lobby by any means
- playerLeave(player.leave)(\[playerName, teamId\])
#### New team
- is sent when a new team is created
- newTeam(name)(\[color\])
#### Change Team
- Is sent when a player changes his team
- changeTeam(name)(\[teamId\])
#### Start game
- When the game begins, all players are sent this packet
- gameStart()(\[\])
