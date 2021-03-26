package module

import (
	"time"
)

// Boss ...
type Boss struct {
	Name         string
	Detail       string
	AttackPower  int
	DefensePower int
	Blood        int
	Level        int
}

// Hero ...
type Hero struct {
	Name         string `json:"name,omitempty"`
	Detail       string
	AttackPower  int
	DefensePower int
	Blood        int
}

// Session ...
type Session struct {
	UID           string
	HeroName      string
	LiveHeroBlood int
	LiveBossBlood int
	CurrentLevel  int
	Score         int
	ArchiveDate   time.Time
}

// SessionView ...
type SessionView struct {
	Session
	Boss
	Hero
}
