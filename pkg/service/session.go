package service

import (
	"errors"
	"sort"

	"sync"

	"github.com/TianqiuHuang/grpc-fight-app/pkg/module"
)

// ErrorNotFound ...
var ErrorNotFound = errors.New("session not found")

type sessions struct {
	signal chan struct{}
	lock   sync.Mutex
	maps   map[string]*module.SessionView
}

var sessionStore = &sessions{
	lock:   sync.Mutex{},
	signal: make(chan struct{}, 1024),
	maps:   make(map[string]*module.SessionView),
}

func (ss *sessions) Remove(id string) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	// ss.cache.Delete(id)
	delete(ss.maps, id)
	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}
}

type player struct {
	id    string
	score int
	level int
}

type collections []player

func (c collections) Len() int {
	return len(c)
}

func (c collections) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c collections) Less(i, j int) bool {
	return c[i].score >= c[j].score
}

func (ss *sessions) ListTop(num int) ([]player, error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	var (
		players = make([]player, len(ss.maps))
		i       = 0
	)

	for id := range ss.maps {
		sessionView := ss.maps[id]
		players[i] = player{
			id:    id,
			score: sessionView.Score,
			level: sessionView.CurrentLevel,
		}
		i++
	}

	sort.Sort(collections(players))
	if len(players) >= num {
		return players[:num], nil
	}

	return players, nil
}

func (ss *sessions) GetCurrentLevel(id string) (int, error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	sessionView, ok := ss.maps[id]
	if !ok {
		return 0, ErrorNotFound
	}
	return sessionView.Session.CurrentLevel, nil
}

func (ss *sessions) GetSession(id string) (*module.Session, error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	sessionView, ok := ss.maps[id]
	if !ok {
		return nil, ErrorNotFound
	}
	return &sessionView.Session, nil
}

func (ss *sessions) Add(id string, sessionView *module.SessionView) error {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	ss.maps[id] = sessionView
	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}

func (ss *sessions) Get(id string) (*module.SessionView, error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	value, ok := ss.maps[id]
	if !ok {
		return nil, ErrorNotFound
	}
	return value, nil
}

func (ss *sessions) Update(id string, s *module.SessionView) error {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	ss.maps[id] = s

	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}

func (ss *sessions) UpdateHero(id string, hero module.Hero) error {
	sessionView, ok := ss.maps[id]
	if !ok {
		return ErrorNotFound
	}

	sessionView.Hero = hero
	sessionView.Session.LiveHeroBlood = hero.Blood
	sessionView.Session.HeroName = hero.Name

	if err := ss.Update(id, sessionView); err != nil {
		return err
	}

	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}
