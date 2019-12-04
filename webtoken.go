package main

import (
	"crypto/rand"
	"encoding/base32"
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

func (w *WebTokenStorage) Expire() uint {
	var count uint
	w.mutex.Lock()
	defer w.mutex.Unlock()
	for k, v := range w.storage {
		count++
		if time.Since(v.Timestamp) > 24*time.Hour {
			delete(w.storage, k)
			count--
		}
	}
	return count
}
