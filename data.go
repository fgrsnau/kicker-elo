package main

type User struct {
	Id                int64
	User, First, Last string
	Elo               float32
}

type Game struct {
	Id            int64
	Front1, Back1 User
	Score1        int
	Front2, Back2 User
	Score2        int
}
