package main

type POI struct {
	id           int
	lat          float64
	lon          float64
	name         string
	locationType string
}

type input struct {
	lat float64
	lon float64
}
