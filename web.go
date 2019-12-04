package main

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"time"
)

type WebToken struct {
	User      string
	Timestamp time.Time
}

func NewWebToken(user string) WebToken {
	return WebToken{
		User:      user,
		Timestamp: time.Now(),
	}
}

func (t WebToken) Refresh() WebToken {
	newToken := t
	newToken.Timestamp = time.Now()
	return newToken
}

type WebTokenStorage struct {
	storage map[string]WebToken
	mutex   sync.Mutex
}

func NewWebTokenStorage() WebTokenStorage {
	return WebTokenStorage{
		storage: make(map[string]WebToken),
	}
}

func (w *WebTokenStorage) computeToken() string {
	var tmp [32]byte
	if _, err := rand.Read(tmp[:]); err != nil {
		panic(err)
	}
	return base32.StdEncoding.EncodeToString(tmp[:])
}

func (w *WebTokenStorage) MapUser(user string) string {
	for {
		token := w.computeToken()
		w.mutex.Lock()
		if _, ok := w.storage[token]; !ok {
			w.storage[token] = NewWebToken(user)
			w.mutex.Unlock()
			return token
		}
		w.mutex.Unlock()
	}
}

func (w *WebTokenStorage) VerifyToken(token string) string {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	t, ok := w.storage[token]
	if ok {
		w.storage[token] = t.Refresh()
		return t.User
	}
	return ""
}

func (w *WebTokenStorage) Expire() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	for k, v := range w.storage {
		if time.Since(v.Timestamp) > 24*time.Hour {
			delete(w.storage, k)
		}
	}
}

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
		User     string
		Password string
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
		Won, Lost, Games  uint32
	}
	var response []ResponseItem

	if user := webParseRequestAndVerifyToken(w, r, &request); user != "" {
		users := Db.GetUsers()
		response = make([]ResponseItem, len(users))
		for i, user := range users {
			response[i] = ResponseItem{
				User:  user.User,
				First: user.First,
				Last:  user.Last,
				Elo:   user.Elo,
				Won:   0,
				Lost:  0,
				Games: 0,
			}
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
							User:  game.Front1.User,
							First: game.Front1.First,
							Last:  game.Front1.Last},
						Back: UserItem{
							User:  game.Back1.User,
							First: game.Back1.First,
							Last:  game.Back1.Last},
					},
					TeamItem{
						Front: UserItem{
							User:  game.Front2.User,
							First: game.Front2.First,
							Last:  game.Front2.Last},
						Back: UserItem{
							User:  game.Back2.User,
							First: game.Back2.First,
							Last:  game.Back2.Last},
					},
				},
				Score: [2]int{game.Score1, game.Score2},
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
		Token, Front1, Back1, Front2, Back2 string
		Score1, Score2                      int
	}

	if tokenUser := webParseRequestAndVerifyToken(w, r, &request); tokenUser != "" {
		var users [5]*User
		usernames := [5]string{tokenUser, request.Front1, request.Front2, request.Back1, request.Back2}
		for i, username := range usernames {
			user, err := Db.GetUser(username)
			if err != nil || user == nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			users[i] = user
		}

		if users[1].Id == users[3].Id || users[1].Id == users[4].Id || users[2].Id == users[3].Id || users[2].Id == users[4].Id {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		game := &Game{
			Front1: *users[1],
			Front2: *users[2],
			Back1:  *users[3],
			Back2:  *users[4],
			Score1: request.Score1,
			Score2: request.Score2,
		}
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
		user, err := Db.GetUser(tokenUser)
		if err != nil {
			panic(err)
		}

		Db.AddSignOff(user, &Game{Id: request.Game}) // FIXME: Quite hacky :-/
		w.WriteHeader(http.StatusOK)
	}
}

func WebRun() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			WebTokens.Expire()
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
