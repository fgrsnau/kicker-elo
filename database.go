package main

import (
	"time"
	"database/sql"
	"log"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseQueryTag uint8

const (
	DatabaseQueryUsers DatabaseQueryTag = iota
	DatabaseQueryUser
	DatabaseQueryLogin
	DatabaseQueryAddUser
	DatabaseQueryGames
	DatabaseQueryGamesAsc
	DatabaseQueryGamesDesc
	DatabaseQueryAddGame
	DatabaseQueryAddSignOff
)

var DatabaseQueryStrings map[DatabaseQueryTag]string

func init() {
	DatabaseQueryStrings = make(map[DatabaseQueryTag]string)

	DatabaseQueryStrings[DatabaseQueryUsers] = `
		SELECT u.id, u.user, u.first, u.last, COALESCE(e.elo, ?), COALESCE(e.won, 0), COALESCE(e.lost, 0), COALESCE(e.games, 0)
		FROM user u INNER JOIN elo e ON e.user = u.id`

	DatabaseQueryStrings[DatabaseQueryUser] = DatabaseQueryStrings[DatabaseQueryUsers] + `
		WHERE u.user=?`

	DatabaseQueryStrings[DatabaseQueryLogin] = `
		SELECT password_hash FROM user WHERE user=?`

	DatabaseQueryStrings[DatabaseQueryAddUser] = `
		INSERT INTO user (user, password_hash, first, last) VALUES (?, ?, ?, ?)`

	DatabaseQueryStrings[DatabaseQueryGames] = `
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

	DatabaseQueryStrings[DatabaseQueryGamesAsc] = DatabaseQueryStrings[DatabaseQueryGames] + `
		ORDER BY g.time ASC`

	DatabaseQueryStrings[DatabaseQueryGamesDesc] = DatabaseQueryStrings[DatabaseQueryGames] + `
		ORDER BY g.time DESC`

	DatabaseQueryStrings[DatabaseQueryAddSignOff] = `
		INSERT INTO signoff (user, game) VALUES (?, ?)`
}

const (
	databaseOptions = "?_busy_timeout=15000&_foreign_keys=1&_journal_mode=WAL"
)

var Db Database

type Database struct {
	Db    *sql.DB
	stmts map[DatabaseQueryTag]*sql.Stmt
}

func (d *Database) Initialize() {
	db, err := sql.Open("sqlite3", "elo.db"+databaseOptions)
	if err != nil {
		panic(err)
	}
	d.Db = db

	d.stmts = make(map[DatabaseQueryTag]*sql.Stmt)
	for tag, query := range DatabaseQueryStrings {
		stmt, err := db.Prepare(query)
		if err != nil {
			panic(err)
		}
		d.stmts[tag] = stmt
	}
}

func (d *Database) GetUsers() <-chan User {
	c := make(chan User, 10)
	go func() {
		rows, err := d.stmts[DatabaseQueryUsers].Query(EloInitialValue)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var user User
			err = rows.Scan(&user.Id, &user.User, &user.First, &user.Last,
				&user.Elo, &user.Won, &user.Lost, &user.Games)
			if err != nil {
				panic(err)
			}
			c <- user
		}
		if err := rows.Err(); err != nil {
			panic(err)
		}
		close(c)
	}()
	return c
}

func (d *Database) GetUser(username string) (user User, ok bool) {
	row := d.stmts[DatabaseQueryUser].QueryRow(EloInitialValue, username)
	err := row.Scan(&user.Id, &user.User, &user.First, &user.Last,
		&user.Elo, &user.Won, &user.Lost, &user.Games)
	switch err {
	case nil:
		ok = true
	case sql.ErrNoRows:
		ok = false
	default:
		panic(err)
	}
	return
}

func (d *Database) Register(user, first, last string, password []byte) bool {
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	_, err = d.stmts[DatabaseQueryAddUser].Exec(user, hash, first, last)
	return err == nil
}

func (d *Database) Login(user string, password []byte) bool {
	row := d.stmts[DatabaseQueryLogin].QueryRow(user)

	var hash []byte
	if err := row.Scan(&hash); err != nil {
		return false
	}

	return bcrypt.CompareHashAndPassword(hash, password) == nil
}

func (d *Database) GetGames(ascending bool) (<-chan Game, chan<- bool) {
	var stmt *sql.Stmt
	if ascending {
		stmt = d.stmts[DatabaseQueryGamesAsc]
	} else {
		stmt = d.stmts[DatabaseQueryGamesDesc]
	}

	c := make(chan Game, 10)
	cAbort := make(chan bool, 1)

	go func() {
		defer close(c)

		rows, err := stmt.Query()
		if err != nil {
			panic(err)
		}
		defer rows.Close()

	row_loop:
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
				break row_loop
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

	res, err := d.stmts[DatabaseQueryAddGame].Exec(
		game.Teams[0].Front.Id, game.Teams[0].Back.Id, game.Score[0],
		game.Teams[1].Front.Id, game.Teams[1].Back.Id, game.Score[1])
	if err != nil {
		panic(err)
	}

	if game.Id, err = res.LastInsertId(); err != nil {
		panic(err)
	}
}

func (d *Database) AddSignOff(user User, game *Game) {
	_, err := d.stmts[DatabaseQueryAddSignOff].Exec(user.Id, game.Id)
	if err != nil {
		panic(err)
	}
}

func (d *Database) StatsRunner() {
	for {
		time.Sleep(10 * time.Minute)
		log.Printf("%+v", Db.Db.Stats())
	}
}
