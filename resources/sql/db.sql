CREATE TABLE Hero (
    Name varchar(20) PRIMARY KEY,
    Detail text check (length(Detail) > 8),
    AttackPower int,
    DefensePower int,
    Blood int
);

INSERT INTO Hero VALUES ('Charlie', 'This is hero Charlie, coll!', 20, 10, 100);
INSERT INTO Hero VALUES ('James', 'This is hero James, cool', 10, 30, 100);


CREATE TABLE Boss (
    Name varchar(20) ,
    Detail text check (length(Detail) > 4),
    AttackPower int,
    DefensePower int,
    Blood int,
    Level int UNIQUE
);

INSERT INTO Boss VALUES ('Boss1','This is Boss1, cool!', 20, 5, 100, 1);
INSERT INTO Boss VALUES ('Boss2','This is Boss2, cool!', 15, 10, 100, 2);

CREATE TABLE Session(
    UID varchar(100) primary key,
    HeroName varchar(20) references hero(name),
    HeroBlood int,
    BossBlood int,
    CurrentLevel int references boss(level),
    Score int,
    ArchiveDate timestamp default now()
);


UPDATE session
SET heroblood = value1, bossblood = value2, currentlevel = value3, score = value4, archivedate = value5
WHERE uid = %s;


create view as session_view
    select from cast(session, hero, boss as )

INSERT INTO Session VALUES ('4','Charlie', 101, 100, 1, 20, '2021-03-11T18:25:06.1577213+08:00');


CREATE VIEW session_view AS
SELECT
    session.uid AS sessionid,
    session.heroname as heroname,
    hero.detail AS hero_detail,
    hero.attackpower as hero_attackpower,
    hero.defensepower as hero_defensepower,
    hero.blood as hero_full_blood,
    session.heroblood as live_hero_blood,
    session.bossblood as live_boss_blood,
    session.currentlevel,
    session.score,
    session.archivedate,
    boss.name as bossname,
    boss.detail as boss_detail,
    boss.attackpower as boss_attackpower,
    boss.defensepower as boss_defensepower,
    boss.blood as boss_full_blood
FROM session, hero, boss
WHERE session.heroname = hero.name and session.currentlevel = boss.level;

CREATE or REPLACE FUNCTION notify_event() RETURNS TRIGGER AS $$
    DECLARE
        data json;
        notification json;
    
    BEGIN
        IF (TG_OP = 'DELETE') THEN
            data = row_to_json(OLD);
        ELSE
            data = row_to_json(NEW);
        END IF;

        notification = json_build_object ('table', TG_TABLE_NAME, 'action', TG_OP, 'data', data);

        PERFORM pg_notify('events', notification::text);
        RETURN NULL;
    END;

$$ LANGUAGE plpgsql;


CREATE TRIGGER hero_notify_event
AFTER INSERT OR UPDATE OR DELETE ON hero
    FOR EACH ROW EXECUTE PROCEDURE notify_event();
