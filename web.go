package main

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"time"
)

var WebTokens WebTokenStorage = NewWebTokenStorage()

func webParseRequest(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func webParseRequestAndVerifyToken(w http.ResponseWriter, r *http.Request, v interface{}) string {
	user := ""
	if webParseRequest(w, r, v) {
		token := reflect.Indirect(reflect.ValueOf(v)).FieldByName("Token").String()
		user = WebTokens.VerifyToken(token)
		if user == "" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}
	return user
}

func webHandleLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		User, Password string
	}

	var response struct {
		Token string
	}

	if webParseRequest(w, r, &request) {
		if Db.Login(request.User, []byte(request.Password)) {
			response.Token = WebTokens.MapUser(request.User)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}
}

func webHandleRegister(w http.ResponseWriter, r *http.Request) {
	var request struct {
		User, Password, First, Last string
	}

	var response struct {
		Token string
	}

	if webParseRequest(w, r, &request) {
		if len(request.User) < 2 || len(request.Password) < 3 || request.First == "" || request.Last == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if Db.Register(request.User, request.First, request.Last, []byte(request.Password)) {
			response.Token = WebTokens.MapUser(request.User)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	}
}

func webHandleUsers(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string
	}

	type ResponseItem struct {
		User, First, Last string
		Elo               float32
		Won, Lost, Games  int
	}
	var response []ResponseItem

	if user := webParseRequestAndVerifyToken(w, r, &request); user != "" {
		userChan := Db.GetUsers()
		response = make([]ResponseItem, 0, 50)
		for user := range userChan {
			response = append(response, ResponseItem{
				User:  user.User,
				First: user.First,
				Last:  user.Last,
				Elo:   user.Elo,
				Won:   user.Won,
				Lost:  user.Lost,
				Games: user.Games,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func webHandleGames(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string
	}

	type UserItem struct {
		User, First, Last string
	}

	type TeamItem struct {
		Front, Back UserItem
	}

	type GameItem struct {
		Id    int64
		Teams [2]TeamItem
		Score [2]int
	}

	if tokenUser := webParseRequestAndVerifyToken(w, r, &request); tokenUser != "" {
		response := make([]GameItem, 0, 25)
		cGame, cAbort := Db.GetGames(false)
		for game := range cGame {
			item := GameItem{
				Id: game.Id,
				Teams: [2]TeamItem{
					TeamItem{
						Front: UserItem{
							User:  game.Teams[0].Front.User,
							First: game.Teams[0].Front.First,
							Last:  game.Teams[0].Front.Last},
						Back: UserItem{
							User:  game.Teams[0].Back.User,
							First: game.Teams[0].Back.First,
							Last:  game.Teams[0].Back.Last},
					},
					TeamItem{
						Front: UserItem{
							User:  game.Teams[1].Front.User,
							First: game.Teams[1].Front.First,
							Last:  game.Teams[1].Front.Last},
						Back: UserItem{
							User:  game.Teams[1].Back.User,
							First: game.Teams[1].Back.First,
							Last:  game.Teams[1].Back.Last},
					},
				},
				Score: game.Score,
			}

			response = append(response, item)
			if len(response) >= cap(response) {
				break
			}
		}
		cAbort <- true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func webHandleAddGame(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string
		Teams [2][2]string
		Score [2]int
	}

	if tokenUser := webParseRequestAndVerifyToken(w, r, &request); tokenUser != "" {
		var users [5]User
		usernames := [5]string{tokenUser, request.Teams[0][0], request.Teams[0][1],
			request.Teams[1][0], request.Teams[1][1]}
		for i, username := range usernames {
			user, ok := Db.GetUser(username)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			users[i] = user
		}

		if users[1].Id == users[3].Id || users[1].Id == users[4].Id ||
			users[2].Id == users[3].Id || users[2].Id == users[4].Id {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		game := &Game{}
		game.Teams[0].Front = users[1]
		game.Teams[0].Back = users[2]
		game.Teams[1].Front = users[3]
		game.Teams[1].Back = users[4]
		game.Score = request.Score
		Db.AddGame(game)
		Db.AddSignOff(users[0], game)
		w.WriteHeader(http.StatusOK)
	}
}

func webHandleAddSignOff(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string
		Game  int64
	}

	if tokenUser := webParseRequestAndVerifyToken(w, r, &request); tokenUser != "" {
		user, ok := Db.GetUser(tokenUser)
		if !ok {
			panic("TokenUser not found")
		}

		Db.AddSignOff(user, &Game{Id: request.Game}) // FIXME: Quite hacky :-/
		w.WriteHeader(http.StatusOK)
	}
}

func WebRun() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			count := WebTokens.Expire()
			log.Printf("current web tokens count: %d", count)
		}
	}()

	http.Handle("/", http.FileServer(http.Dir("web")))
	http.HandleFunc("/api/v1/login", webHandleLogin)
	http.HandleFunc("/api/v1/register", webHandleRegister)
	http.HandleFunc("/api/v1/users", webHandleUsers)
	http.HandleFunc("/api/v1/games", webHandleGames)
	http.HandleFunc("/api/v1/add_game", webHandleAddGame)
	http.HandleFunc("/api/v1/add_signoff", webHandleAddSignOff)
	http.ListenAndServe(":8080", nil)
}
