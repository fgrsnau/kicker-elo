package main

import (
	"database/sql"
	"math"
	"time"
)

const EloInitialValue float64 = 500

func EloRunner() {
	for {
		if err := EloRecompute(); err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Minute)
	}
}

func EloRecompute() (err error) {
	tx, err := Db.Db.Begin()
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
	}()

	if err = eloInitializeDatabase(tx); err != nil {
		return
	}

	cGame, _ := Db.GetGames(true)
	for game := range cGame {
		eloProcessGame(tx, game)
	}

	err = tx.Commit()
	return
}

func eloInitializeDatabase(tx *sql.Tx) (err error) {
	_, err = tx.Exec("DELETE FROM elo")
	if err != nil {
		return
	}

	_, err = tx.Exec("INSERT INTO elo (user, elo, games) SELECT id, 500.0, 0 FROM user")
	if err != nil {
		return
	}

	return
}

func eloProcessGame(tx *sql.Tx, game Game) (err error) {
	expected, err := eloExpectedResult(tx, game)
	if err != nil {
		return
	}

	result := eloComputeResult(game)
	change := eloGoalFactor(game) * (result - expected)

	changes := make(map[int64]float64)
	changes[game.Teams[0].Front.Id] = change
	changes[game.Teams[0].Back.Id] = change
	changes[game.Teams[1].Front.Id] = -change
	changes[game.Teams[1].Back.Id] = -change

	for user, change := range changes {
		var k float64
		k, err = eloKFactor(tx, user)
		if err != nil {
			return
		}

		err = eloUpdate(tx, user, k*change)
		if err != nil {
			return
		}
	}

	return
}

func eloComputeResult(game Game) float64 {
	if game.Score[0] < game.Score[1] {
		return 0
	}
	if game.Score[0] > game.Score[1] {
		return 1
	}
	return 0.5
}

func eloExpectedResult(tx *sql.Tx, game Game) (float64, error) {
	var elo [4]float64
	users := [4]int64{
		game.Teams[0].Front.Id,
		game.Teams[0].Back.Id,
		game.Teams[1].Front.Id,
		game.Teams[1].Back.Id}
	for i, user := range users {
		row := tx.QueryRow("SELECT elo FROM elo WHERE user=?", user)
		if err := row.Scan(&elo[i]); err != nil {
			return 0, err
		}
	}

	team0 := (elo[0] + elo[1]) * 0.5
	team1 := (elo[2] + elo[3]) * 0.5
	return 1.0 / (math.Pow(10, (team1-team0)/400.0) + 1), nil
}

func eloGoalFactor(game Game) float64 {
	scoreDiff := game.Score[1] - game.Score[0]
	if scoreDiff < 0 {
		scoreDiff = -scoreDiff
	}

	switch scoreDiff {
	case 0:
		return 1.0
	case 1:
		return 1.0
	case 2:
		return 1.0
	case 3:
		return 1.33
	case 4:
		return 1.66
	default:
		return 2.0
	}
}

func eloKFactor(tx *sql.Tx, user int64) (float64, error) {
	var games int
	row := tx.QueryRow("SELECT games FROM elo WHERE user=?", user)
	if err := row.Scan(&games); err != nil {
		return 0, err
	}

	if games > 10 {
		return 25.0, nil
	}

	return (1.0-float64(games)/10)*10.0 + 25.0, nil
}

func eloUpdate(tx *sql.Tx, user int64, update float64) (err error) {
	_, err = tx.Exec("UPDATE elo SET elo=elo+?, games=games+1 WHERE user=?",
		update, user)
	return
}
