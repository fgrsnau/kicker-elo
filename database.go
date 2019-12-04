package main

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
)

var Db Database

type Database struct {
	Db *sql.DB
}

func (d *Database) Initialize() {
	db, err := sql.Open("sqlite3", "elo.db?_journal_mode=WAL&foreign_keys=1&_busy_timeout=30000")
	if err != nil {
		panic(err)
	}
	d.Db = db
}

func (d *Database) GetUsers() []*User {
	rows, err := d.Db.Query(
		`SELECT u.id, u.user, u.first, u.last, COALESCE(e.elo, ?)
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
		if err := rows.Scan(&user.Id, &user.User, &user.First, &user.Last, &user.Elo); err != nil {
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
		SELECT u.id, u.user, u.first, u.last, COALESCE(e.elo, ?)
		FROM user u
		INNER JOIN elo e ON e.user = u.id
		WHERE u.user=?`, EloInitialValue, user)

	var result User
	if err := row.Scan(&result.Id, &result.User, &result.First, &result.Last, &result.Elo); err != nil {
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
				&game.Id, &game.Score1, &game.Score2,
				&game.Front1.Id, &game.Front1.User, &game.Front1.First, &game.Front1.Last,
				&game.Back1.Id, &game.Back1.User, &game.Back1.First, &game.Back1.Last,
				&game.Front2.Id, &game.Front2.User, &game.Front2.First, &game.Front2.Last,
				&game.Back2.Id, &game.Back2.User, &game.Back2.First, &game.Back2.Last)
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
		game.Front1.Id, game.Back1.Id, game.Score1, game.Front2.Id, game.Back2.Id, game.Score2)
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
