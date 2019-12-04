package main

type User struct {
	Id                int64
	User, First, Last string
	Elo               float32
	Won, Lost, Games  int
}

type Team struct {
	Front, Back User
}

type Game struct {
	Id    int64
	Teams [2]Team
	Score [2]int
}
