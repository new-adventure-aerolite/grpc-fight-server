package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"sync"

	"github.com/TianqiuHuang/grpc-fight-app/pkg/module"
	gc "github.com/patrickmn/go-cache"
)

// ErrorNotFound ...
var ErrorNotFound = errors.New("session not found")

type sessions struct {
	cache  *gc.Cache
	signal chan struct{}
	lock   sync.Mutex
}

var sessionStore = &sessions{
	cache:  gc.New(5*time.Minute, 10*time.Minute),
	lock:   sync.Mutex{},
	signal: make(chan struct{}, 1024),
}

func (ss *sessions) Remove(id string) {
	ss.cache.Delete(id)

	ss.lock.Lock()
	defer ss.lock.Unlock()
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
	var (
		items   = ss.cache.Items()
		players = make([]player, len(items))
		i       = 0
	)

	for id := range items {
		sessionView, ok := items[id].Object.(*module.SessionView)
		if !ok {
			return nil, fmt.Errorf("expect seesion view, but got: '%T'", items[id].Object)
		}
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
	sessionView, err := ss.Get(id)
	if err != nil {
		return 0, err
	}
	return sessionView.Session.CurrentLevel, nil
}

func (ss *sessions) GetSession(id string) (*module.Session, error) {
	sessionView, err := ss.Get(id)
	if err != nil {
		return nil, err
	}
	return &sessionView.Session, nil
}

func (ss *sessions) Add(id string, sessionView *module.SessionView) error {
	err := ss.cache.Add(id, sessionView, gc.NoExpiration)
	if err != nil {
		return err
	}

	ss.lock.Lock()
	defer ss.lock.Unlock()
	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}

func (ss *sessions) Get(id string) (*module.SessionView, error) {
	value, ok := ss.cache.Get(id)
	if !ok {
		return nil, ErrorNotFound
	}

	sessionView, ok := value.(*module.SessionView)
	if !ok {
		return nil, fmt.Errorf("expect seesion view, but got: '%T'", value)
	}

	return sessionView, nil
}

func (ss *sessions) Update(id string, s *module.SessionView) error {
	if err := ss.cache.Replace(id, s, gc.NoExpiration); err != nil {
		return err
	}

	ss.lock.Lock()
	defer ss.lock.Unlock()
	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}

func (ss *sessions) UpdateHero(id string, hero module.Hero) error {
	sessionView, err := ss.Get(id)
	if err != nil {
		return err
	}

	sessionView.Hero = hero
	sessionView.Session.LiveHeroBlood = hero.Blood
	sessionView.Session.HeroName = hero.Name

	if err = ss.Update(id, sessionView); err != nil {
		return err
	}

	ss.lock.Lock()
	defer ss.lock.Unlock()
	if len(ss.signal) < 1024 {
		ss.signal <- struct{}{}
	}

	return nil
}
