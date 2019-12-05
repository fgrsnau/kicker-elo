package main

import (
	"database/sql"
	"strings"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
)

var Db Database

type Database struct {
	Db *sql.DB
}

func (d *Database) Initialize() {
	options := [...]string{
		"_busy_timeout=15000",
		"_foreign_keys=1",
		"_journal_mode=WAL"}
	connect_string := "elo.db?" + strings.Join(options[:], "&")
	db, err := sql.Open("sqlite3", connect_string)
	if err != nil {
		panic(err)
	}
	d.Db = db
}

func (d *Database) GetUsers() []*User {
	rows, err := d.Db.Query(
		`SELECT u.id, u.user, u.first, u.last, COALESCE(e.elo, ?), COALESCE(e.won, 0), COALESCE(e.lost, 0), COALESCE(e.games, 0)
		FROM user u
		INNER JOIN elo e ON e.user = u.id`,
		EloInitialValue)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Id, &user.User, &user.First, &user.Last, &user.Elo, &user.Won, &user.Lost, &user.Games); err != nil {
			panic(err)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}

	return users
}

func (d *Database) GetUser(user string) (*User, error) {
	row := d.Db.QueryRow(`
		SELECT u.id, u.user, u.first, u.last, COALESCE(e.elo, ?), COALESCE(e.won, 0), COALESCE(e.lost, 0), COALESCE(e.games, 0)
		FROM user u
		INNER JOIN elo e ON e.user = u.id
		WHERE u.user=?`, EloInitialValue, user)

	var result User
	if err := row.Scan(&result.Id, &result.User, &result.First, &result.Last, &result.Elo, &result.Won, &result.Lost, &result.Games); err != nil {
		return nil, err
	}

	return &result, nil
}

func (d *Database) Register(user, first, last string, password []byte) bool {
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	_, err = d.Db.Exec("INSERT INTO user (user, password_hash, first, last) VALUES (?, ?, ?, ?)",
		user, hash, first, last)
	return err == nil
}

func (d *Database) Login(user string, password []byte) bool {
	row := d.Db.QueryRow("SELECT password_hash FROM user WHERE user=?", user)

	var hash []byte
	if err := row.Scan(&hash); err != nil {
		return false
	}

	return bcrypt.CompareHashAndPassword(hash, password) == nil
}

func (d *Database) GetGames(ascending bool) (<-chan Game, chan<- bool) {
	query := `
		SELECT
			g.id, g.score1, g.score2,
			f1.id, f1.user, f1.first, f1.last,
			b1.id, b1.user, b1.first, b1.last,
			f2.id, f2.user, f2.first, f2.last,
			b2.id, b2.user, b2.first, b2.last
		FROM game g
		INNER JOIN user f1 ON f1.id == g.front1
		INNER JOIN user b1 ON b1.id == g.back1
		INNER JOIN user f2 ON f2.id == g.front2
		INNER JOIN user b2 ON b2.id == g.back2`
	if ascending {
		query += "\nORDER BY g.time ASC"
	} else {
		query += "\nORDER BY g.time DESC"
	}

	c := make(chan Game)
	cAbort := make(chan bool, 1)

	go func() {
		defer close(c)

		rows, err := d.Db.Query(query)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var game Game
			err := rows.Scan(
				&game.Id, &game.Score[0], &game.Score[1],
				&game.Teams[0].Front.Id, &game.Teams[0].Front.User, &game.Teams[0].Front.First, &game.Teams[0].Front.Last,
				&game.Teams[0].Back.Id, &game.Teams[0].Back.User, &game.Teams[0].Back.First, &game.Teams[0].Back.Last,
				&game.Teams[1].Front.Id, &game.Teams[1].Front.User, &game.Teams[1].Front.First, &game.Teams[1].Front.Last,
				&game.Teams[1].Back.Id, &game.Teams[1].Back.User, &game.Teams[1].Back.First, &game.Teams[1].Back.Last)
			if err != nil {
				panic(err)
			}

			select {
			case c <- game:
			case <-cAbort:
				break
			}
		}
		if err := rows.Err(); err != nil {
			panic(err)
		}
	}()

	return c, cAbort
}

func (d *Database) AddGame(game *Game) {
	if game.Id > 0 {
		panic("Game already present in database.")
	}

	res, err := d.Db.Exec("INSERT INTO game (front1, back1, score1, front2, back2, score2) VALUES (?, ?, ?, ?, ?, ?)",
		game.Teams[0].Front.Id, game.Teams[0].Back.Id, game.Score[0], game.Teams[1].Front.Id, game.Teams[1].Back.Id, game.Score[1])
	if err != nil {
		panic(err)
	}

	if game.Id, err = res.LastInsertId(); err != nil {
		panic(err)
	}
}

func (d *Database) AddSignOff(user *User, game *Game) {
	d.Db.Exec("INSERT INTO signoff (user, game) VALUES (?, ?)",
		user.Id, game.Id)
}
