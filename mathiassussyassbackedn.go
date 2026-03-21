package main

type WaypointQuests struct {
	waypoinId      int
	waypointQuests []WaypointQuest
}

type QuestType int

const (
	Input QuestType = iota
	MultipleSelect
	SingleSelect
)

type WaypointQuest struct {
	timeLimit      int
	message        string
	questType      QuestType
	correctAnswers []string
	answerOptions  []string
}

type RouteWaypointQuest struct {
	routeId             int
	routeWaypointQuests []WaypointQuests
}
