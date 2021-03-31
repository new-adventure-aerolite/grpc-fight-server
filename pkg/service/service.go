package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"sync"

	"github.com/TianqiuHuang/grpc-fight-app/pd/fight"
	"github.com/TianqiuHuang/grpc-fight-app/pkg/module"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	tags "github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog"
)

type GameError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (err GameError) Error() string {
	return fmt.Sprintf("code: '%d', msg: '%s'", err.Code, err.Msg)
}

var ErrNoBossExistsForLevel = GameError{
	Msg:  "no boss exists for the level",
	Code: 404,
}

var once = sync.Once{}

// Service implements FightSvcServer interface.
type Service struct {
	db       *sql.DB
	listener *pq.Listener
	tracer   opentracing.Tracer
}

// New creates a new service.
func New(db *sql.DB, ls *pq.Listener, tracer opentracing.Tracer) *Service {
	return &Service{
		db:       db,
		listener: ls,
		tracer:   tracer,
	}
}

// Event ...
type Event struct {
	Table  string      `json:"table"`
	Action string      `json:"action"`
	Data   module.Hero `json:"data"`
}

func (s *Service) ClearSession(ctx context.Context, req *fight.ClearSessionRequest) (*fight.ClearSessionResponse, error) {
	id := req.GetId()
	sessionStore.Remove(id)
	if err := s.removeSessionFromDB(id, ctx); err != nil {
		return &fight.ClearSessionResponse{}, err
	}
	return &fight.ClearSessionResponse{
		Msg: "data cleared",
	}, nil
}

// Admin ...
func (s *Service) Admin(stream fight.FightSvc_AdminServer) error {
	var f = func() error {
		once.Do(func() {
			if err := s.listener.Listen("events"); err != nil {
				klog.Warning(err)
			}
		})
		for event := range s.listener.Notify {
			var e Event
			if err := json.Unmarshal([]byte(event.Extra), &e); err != nil {
				return err
			}

			stream.Send(&fight.AdminResponse{
				Heros: []*fight.Hero{
					convertModuleHero2FightHero(e.Data),
				},
			})
		}
		return nil
	}

	go func() {
		if err := f(); err != nil {
			klog.Warning(err)
		}
	}()

	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		switch req.GetType() {
		case fight.AdminRequest_CREATE_HERO:
			for _, hero := range req.GetHeros() {
				if _, err = s.insertHero(hero); err != nil {
					return err
				}
			}

		case fight.AdminRequest_ADJUST_HERO:
			sqlStatement := `UPDATE hero SET attackpower = attackpower*1.2, defensepower = defensepower*1.2;`
			if _, err = s.db.Exec(sqlStatement); err != nil {
				return err
			}

		default:
			return fmt.Errorf("undefined admin request type: '%v'", req.GetType())
		}
	}
}

// Top10 ...
func (s *Service) Top10(req *fight.Top10Request, stream fight.FightSvc_Top10Server) error {
	for range sessionStore.signal {
		players, err := sessionStore.ListTop(10)
		if err != nil {
			return err
		}
		resp := &fight.Top10Response{}
		resp.Players = make([]*fight.Top10Response_Player, len(players))
		for i := 0; i < len(players); i++ {
			resp.Players[i] = &fight.Top10Response_Player{
				Id:    players[i].id,
				Score: int32(players[i].score),
				Level: int32(players[i].level),
			}
		}
		if err = stream.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

// Game ...
func (s *Service) Game(ctx context.Context, req *fight.GameRequest) (*fight.GameResponse, error) {
	var (
		id        = req.GetId()
		eventType = req.GetType()
	)

	sv, err := sessionStore.Get(id)
	if err != nil {
		return &fight.GameResponse{}, err
	}

	switch eventType {
	case fight.Type_ARCHIVE:
		session := sv.Session
		if err = s.archive(session, ctx); err != nil {
			return &fight.GameResponse{}, err
		}
		return &fight.GameResponse{
			Type: eventType,
			Value: &fight.GameResponse_Archive{
				Archive: &fight.Archive{
					Msg:       "Session archived successfully",
					SessionId: id,
				},
			},
		}, nil

	case fight.Type_FIGHT:
		if sv.LiveBossBlood <= 0 || sv.LiveHeroBlood <= 0 {
			return &fight.GameResponse{}, fmt.Errorf("GameOver or NextLevel")
		}
		if sv.Hero.AttackPower >= sv.Boss.DefensePower {
			sv.Session.LiveBossBlood -= (sv.Hero.AttackPower - sv.Boss.DefensePower)
		}
		if sv.Boss.AttackPower >= sv.Hero.DefensePower {
			sv.Session.LiveHeroBlood -= (sv.Boss.AttackPower - sv.Hero.DefensePower)
		}
		// sv.Session.LiveHeroBlood -= sv.Boss.AttackPower
		// sv.Session.LiveBossBlood -= sv.Hero.AttackPower
		sv.Score += 10

		var resp *fight.GameResponse

		if sv.Session.LiveHeroBlood <= 0 {
			sv.Session.LiveHeroBlood = 0
			resp = &fight.GameResponse{
				Type: eventType,
				Value: &fight.GameResponse_Fight{
					Fight: &fight.Fight{
						GameOver:  true,
						NextLevel: false,
						Score:     int32(sv.Score),
						HeroBlood: int32(sv.LiveHeroBlood),
						BossBlood: int32(sv.LiveBossBlood),
					},
				},
			}
			sessionStore.Remove(id)
			if err = s.removeSessionFromDB(id, ctx); err != nil {
				return &fight.GameResponse{}, err
			}
		} else if sv.Session.LiveBossBlood <= 0 {
			sv.Session.LiveBossBlood = 0
			resp = &fight.GameResponse{
				Type: eventType,
				Value: &fight.GameResponse_Fight{
					Fight: &fight.Fight{
						GameOver:  false,
						NextLevel: true,
						Score:     int32(sv.Score),
						HeroBlood: int32(sv.LiveHeroBlood),
						BossBlood: int32(sv.LiveBossBlood),
					},
				},
			}
		} else {
			resp = &fight.GameResponse{
				Type: eventType,
				Value: &fight.GameResponse_Fight{
					Fight: &fight.Fight{
						GameOver:  false,
						NextLevel: false,
						Score:     int32(sv.Score),
						HeroBlood: int32(sv.LiveHeroBlood),
						BossBlood: int32(sv.LiveBossBlood),
					},
				},
			}
		}
		sessionStore.Update(id, sv)
		return resp, nil

	case fight.Type_LEVEL:
		boss, err := s.loadBossFromDB(sv.CurrentLevel+1, ctx)
		if err != nil {
			return &fight.GameResponse{}, err
		}
		sv.Boss = boss
		sv.LiveBossBlood = boss.Blood
		sv.CurrentLevel++
		if err = sessionStore.Update(id, sv); err != nil {
			return &fight.GameResponse{}, err
		}
		return &fight.GameResponse{
			Type: eventType,
			Value: &fight.GameResponse_Level{
				Level: &fight.Level{
					Msg:     "Go to the next Level!",
					Session: convertModuleSession2FightSession(sv.Session),
				},
			},
		}, nil

	case fight.Type_QUIT:
		session := sv.Session
		if err = s.archive(session, ctx); err != nil {
			return &fight.GameResponse{}, err
		}
		sessionStore.Remove(id)
		return &fight.GameResponse{
			Type: eventType,
			Value: &fight.GameResponse_Quit{
				Quit: &fight.Quit{
					Msg: "Archived and quit successfully",
				},
			},
		}, nil

	default:
		return &fight.GameResponse{}, fmt.Errorf("undefined event type: %v", eventType)
	}
}

// SelectHero ...
func (s *Service) SelectHero(ctx context.Context, req *fight.SelectHeroRequest) (*fight.SessionView, error) {
	var (
		id       = req.GetId()
		heroName = req.GetHeroName()
	)

	hero, err := s.loadHeroFromDB(heroName, ctx)
	if err != nil {
		return &fight.SessionView{}, err
	}

	if err = sessionStore.UpdateHero(id, hero); err != nil {
		return &fight.SessionView{}, err
	}

	sv, err := sessionStore.Get(id)
	if err != nil {
		return &fight.SessionView{}, err
	}

	return convertSV2FightSV(*sv), nil
}

// LoadSession ...
func (s *Service) LoadSession(ctx context.Context, req *fight.LoadSessionRequest) (*fight.SessionView, error) {
	var id = req.GetId()
	fmt.Printf("get request: 'LoadSession', id: '%s'\n", id)

	sessionView, err := sessionStore.Get(id)
	switch {
	case err == ErrorNotFound:
		fmt.Printf("session view is not found in the cache: id: '%s'\n", id)
		break

	case err != nil:
		return &fight.SessionView{}, err

	default:
		return convertSV2FightSV(*sessionView), nil
	}

	var ssView = module.SessionView{}

	// simulate opentracing instrumentation of an SQL query
	if span := opentracing.SpanFromContext(ctx); span != nil {
		span := s.tracer.StartSpan("SQL SELECT FROM session_view", opentracing.ChildOf(span.Context()))
		tags.SpanKindRPCServer.Set(span)
		tags.PeerService.Set(span, "postgresql")
		// #nosec
		span.SetTag("sql.query", "SELECT * FROM session_view WHERE sessionid = "+id)
		defer span.Finish()
		ctx = opentracing.ContextWithSpan(ctx, span)
	}

	rows, err := s.db.Query(fmt.Sprintf("SELECT * FROM session_view WHERE sessionid = '%s';", id))
	if err != nil {
		if span := opentracing.SpanFromContext(ctx); span != nil {
			tags.Error.Set(span, true)
		}
		return &fight.SessionView{}, err
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(
			&ssView.Session.UID,
			&ssView.Session.HeroName,
			&ssView.Hero.Detail,
			&ssView.Hero.AttackPower,
			&ssView.Hero.DefensePower,
			&ssView.Hero.Blood,
			&ssView.Session.LiveHeroBlood,
			&ssView.Session.LiveBossBlood,
			&ssView.Session.CurrentLevel,
			&ssView.Session.Score,
			&ssView.Session.ArchiveDate,
			&ssView.Boss.Name,
			&ssView.Boss.Detail,
			&ssView.Boss.AttackPower,
			&ssView.Boss.DefensePower,
			&ssView.Boss.Blood,
		)

		if err != nil {
			return &fight.SessionView{}, err
		}

		ssView.Hero.Name = ssView.Session.HeroName
		ssView.Boss.Level = ssView.Session.CurrentLevel

	} else {
		fmt.Printf("session view is not found in the db: id: '%s'\n", id)
		bossLevel1, err := s.loadBossFromDB(1, ctx)
		if err != nil {
			return &fight.SessionView{}, err
		}
		ssView = module.SessionView{
			Hero: module.Hero{},
			Boss: bossLevel1,
			Session: module.Session{
				UID:           id,
				LiveBossBlood: bossLevel1.Blood,
				CurrentLevel:  bossLevel1.Level,
				ArchiveDate:   time.Now(),
			},
		}
	}

	if err = sessionStore.Add(id, &ssView); err != nil {
		return &fight.SessionView{}, err
	}

	return convertSV2FightSV(ssView), nil
}

// ListHeros ...
func (s *Service) ListHeros(req *fight.ListHerosRequest, stream fight.FightSvc_ListHerosServer) error {
	rows, err := s.db.Query("SELECT * FROM Hero;")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var hero module.Hero
		if err = rows.Scan(&hero.Name, &hero.Detail, &hero.AttackPower, &hero.DefensePower, &hero.Blood); err != nil {
			return err
		}

		if err = stream.Send(convertModuleHero2FightHero(hero)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) insertHero(hero *fight.Hero) (string, error) {
	sqlStatement := `INSERT INTO hero VALUES ($1, $2, $3, $4, $5) RETURNING name;`
	var name string
	err := s.db.QueryRow(sqlStatement,
		hero.Name,
		hero.Details,
		hero.AttackPower,
		hero.DefensePower,
		hero.Blood,
	).Scan(&name)
	return name, err
}

func (s *Service) removeSessionFromDB(id string, ctx context.Context) error {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		childSpan := s.tracer.StartSpan("SQL DELETE FROM session", opentracing.ChildOf(span.Context()))
		tags.SpanKindRPCServer.Set(childSpan)
		tags.PeerService.Set(childSpan, "postgresql")
		childSpan.SetTag("sql.query", "DELETE FROM session where uid = "+id)
		defer childSpan.Finish()
	}

	sqlStatement := "DELETE FROM session where uid = $1;"
	_, err := s.db.Exec(sqlStatement, id)
	return err
}

func (s *Service) archive(session module.Session, ctx context.Context) error {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		childSpan := s.tracer.StartSpan("SQL INSERT TO session", opentracing.ChildOf(span.Context()))
		tags.SpanKindRPCServer.Set(childSpan)
		tags.PeerService.Set(childSpan, "postgresql")
		childSpan.SetTag("sql.query", "INSERT INTO session(uid, heroname, heroblood, bossblood, currentlevel, score, archivedate) VALUES...")
		defer childSpan.Finish()
	}

	sqlStatement := `INSERT INTO session(uid, heroname, heroblood, bossblood, currentlevel, score, archivedate) VALUES($1, $2, $3, $4, $5, $6, $7) ON conflict (uid) DO UPDATE SET heroblood = $8, bossblood = $9, currentlevel = $10, score = $11, archivedate = $12;`
	_, err := s.db.Exec(sqlStatement,
		session.UID,
		session.HeroName,
		session.LiveHeroBlood,
		session.LiveBossBlood,
		session.CurrentLevel,
		session.Score,
		time.Now(),
		session.LiveHeroBlood,
		session.LiveBossBlood,
		session.CurrentLevel,
		session.Score,
		time.Now(),
	)
	return err
}

func (s *Service) loadHeroFromDB(heroName string, ctx context.Context) (module.Hero, error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		childSpan := s.tracer.StartSpan("SQL SELECT FROM hero", opentracing.ChildOf(span.Context()))
		tags.SpanKindRPCServer.Set(childSpan)
		tags.PeerService.Set(childSpan, "postgresql")
		childSpan.SetTag("sql.query", fmt.Sprintf("SELECT * FROM hero WHERE name = '%s';", heroName))
		defer childSpan.Finish()
	}

	var h = module.Hero{}
	rows, err := s.db.Query(fmt.Sprintf("SELECT * FROM hero WHERE name = '%s';", heroName))
	if err != nil {
		return module.Hero{}, err
	}
	defer rows.Close()
	rows.Next()
	err = rows.Scan(&h.Name, &h.Detail, &h.AttackPower, &h.DefensePower, &h.Blood)
	return h, err
}

func (s *Service) loadBossFromDB(level int, ctx context.Context) (module.Boss, error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		childSpan := s.tracer.StartSpan("SQL SELECT FROM boss", opentracing.ChildOf(span.Context()))
		tags.SpanKindRPCServer.Set(childSpan)
		tags.PeerService.Set(childSpan, "postgresql")
		childSpan.SetTag("sql.query", fmt.Sprintf("SELECT * FROM boss WHERE level = %d;", level))
		defer childSpan.Finish()
	}

	var b = module.Boss{}
	rows, err := s.db.Query(fmt.Sprintf("SELECT * FROM boss WHERE level = %d;", level))
	if err != nil {
		return module.Boss{}, err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&b.Name, &b.Detail, &b.AttackPower, &b.DefensePower, &b.Blood, &b.Level)
		return b, err
	} else {
		return module.Boss{}, ErrNoBossExistsForLevel
	}
}

func convertSV2FightSV(sessionView module.SessionView) *fight.SessionView {
	return &fight.SessionView{
		Hero:    convertModuleHero2FightHero(sessionView.Hero),
		Boss:    convertModuleBoss2FightBoss(sessionView.Boss),
		Session: convertModuleSession2FightSession(sessionView.Session),
	}
}

func convertModuleSession2FightSession(session module.Session) *fight.Session {
	return &fight.Session{
		UID:           session.UID,
		HeroName:      session.HeroName,
		LiveHeroBlood: int32(session.LiveHeroBlood),
		LiveBossBlood: int32(session.LiveBossBlood),
		CurrentLevel:  int32(session.CurrentLevel),
		Score:         int32(session.Score),
		ArchiveDate:   timestamppb.New(session.ArchiveDate),
	}
}

func convertModuleBoss2FightBoss(boss module.Boss) *fight.Boss {
	return &fight.Boss{
		Name:         boss.Name,
		Details:      boss.Detail,
		AttackPower:  int32(boss.AttackPower),
		DefensePower: int32(boss.DefensePower),
		Blood:        int32(boss.Blood),
		Level:        int32(boss.Level),
	}
}

func convertModuleHero2FightHero(hero module.Hero) *fight.Hero {
	return &fight.Hero{
		Name:         hero.Name,
		Details:      hero.Detail,
		AttackPower:  int32(hero.AttackPower),
		DefensePower: int32(hero.DefensePower),
		Blood:        int32(hero.Blood),
	}
}
