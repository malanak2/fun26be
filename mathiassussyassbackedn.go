package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Language type and "enum"
type Language string

const (
	English  Language = "English"
	Czech    Language = "Czech"
	Ukranian Language = "Ukranian"
)

// QuestType enum using iota
type QuestType int

const (
	Input QuestType = iota
	MultipleSelect
	SingleSelect
)

// Waypoint struct
type Waypoint struct {
	ID           int     `json:"id"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Name         string  `json:"name"`
	LocationType string  `json:"locationType"`
	Order        int     `json:"order"`
}

// WaypointQuest struct
type WaypointQuest struct {
	TimeLimit      int       `json:"timeLimit"`
	Message        string    `json:"message"`
	QuestType      QuestType `json:"questType"`
	CorrectAnswers []string  `json:"correctAnswers"`
	AnswerOptions  []string  `json:"answerOptions"`
}

// WaypointQuests struct
type WaypointQuests struct {
	WaypointID     int             `json:"waypoinId"` // Kept the source typo 'waypoinId'
	WaypointQuests []WaypointQuest `json:"waypointQuests"`
}

// RouteWaypointQuest struct
type RouteWaypointQuest struct {
	RouteID             int              `json:"routeId"`
	RouteWaypointQuests []WaypointQuests `json:"routeWaypointQuests"`
}

// InputJsonQuestions struct
type InputJsonQuestions struct {
	Lang   Language              `json:"lang"`
	Routes map[string][]Waypoint `json:"routes"`
}

// Coordinate represents a simple geographic point
type Coordinate struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

// POI represents a Point of Interest
type POI struct {
	ID           int      `json:"id"`
	Lat          float64  `json:"lat"`
	Lon          float64  `json:"lon"`
	Name         *string  `json:"name,omitempty"` // Pointer allows for null/nil
	LocationType *string  `json:"locationType,omitempty"`
	Progression  *float64 `json:"progression,omitempty"` // Distance in km
	DistToPath   *float64 `json:"distToPath,omitempty"`  // Distance in km
}

// EnrichedRoutePoint embeds POI to mimic TypeScript's "extends"
type EnrichedRoutePoint struct {
	POI                  // Embedding fields from POI
	ID           int     `json:"id"` // Overriding to ensure they are required
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Name         string  `json:"name"` // Non-pointer means it's required in Go
	LocationType string  `json:"locationType"`
	Order        int     `json:"order"`
}

// RouteStats contains distance information for teams
type RouteStats struct {
	TeamAKm string `json:"teamA_km"`
	TeamBKm string `json:"teamB_km"`
}

// BalancedRouteResponse handles the "0" and "1" keys
type BalancedRouteResponse struct {
	Team0 []EnrichedRoutePoint `json:"0"`
	Team1 []EnrichedRoutePoint `json:"1"`
	Stats RouteStats           `json:"stats"`
}

type InputJsonRoutes struct {
	// Reusing the Coordinate struct for Start and End
	Start             Coordinate `json:"start"`
	End               Coordinate `json:"end"`
	NumberOfWaypoints int        `json:"numberOfWaypoints"`
}

func FetchQuestions(r InputJsonRoutes) (*BalancedRouteResponse, *[]RouteWaypointQuest) {
	m, err := json.Marshal(r)
	if err != nil {
		slog.Warn("Failed to marshal IJS", "ijs", r)
		return nil, nil
	}
	jsonBody := []byte(m)
	bodyReader := bytes.NewReader(jsonBody)
	res, err := http.Post(MathiasLink+"/waypoints", "application/json", bodyReader)
	if err != nil {
		slog.Warn("Failed to post to /waypoints" + err.Error())
		return nil, nil
	}
	slog.Info("Before parse", "parse", res.Body)
	defer res.Body.Close()
	post := &BalancedRouteResponse{}
	derr := json.NewDecoder(res.Body).Decode(&post)
	if derr != nil {
		slog.Error("Failed to decode whateve" + derr.Error())
		return nil, nil
	}
	slog.Info("Post", "post", post)
	m, err = json.Marshal(post)
	if err != nil {
		slog.Warn("Failed to marshal IJS", "ijs", r)
		return nil, nil
	}
	jsonBody = []byte(m)
	bodyReader = bytes.NewReader(jsonBody)
	res, err = http.Post(MathiasLink+"/questions", "application/json", bodyReader)
	if err != nil {
		slog.Warn("Failed to post to /questions" + err.Error())
		return nil, nil
	}
	defer res.Body.Close()
	post2 := &[]RouteWaypointQuest{}
	derr = json.NewDecoder(res.Body).Decode(&post2)
	if derr != nil {
		slog.Error("Failed to decode []RWQ")
		return nil, nil
	}
	return post, post2
}
