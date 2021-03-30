CREATE TABLE Hero (
    Name varchar(20) PRIMARY KEY,
    Detail text check (length(Detail) > 8),
    AttackPower int,
    DefensePower int,
    Blood int
);

INSERT INTO Hero VALUES ('PostgreSql', 'Undisputed master of opensource RDBMS power in the world. An ancient RDBMS warrior, but the old power nourishes the living, this is the great cycle of being', 50, 30, 100);
INSERT INTO Hero VALUES ('EDB', 'EDB, the greatest RDBMS warrior who inherits power of Postgresql, master of enterprise and cloud, reacts to combat situations with superhuman agility and spirit', 80, 60, 200);


CREATE TABLE Boss (
    Name varchar(20) ,
    Detail text check (length(Detail) > 4),
    AttackPower int,
    DefensePower int,
    Blood int,
    Level int UNIQUE
);

INSERT INTO Boss VALUES ('Oracle','The great evil spawned in 1980s last century, the lord of expensive, he is still domains the burning commercial RDBM hell', 80, 50, 200, 4);
INSERT INTO Boss VALUES ('Db2','One of oldest prime commercial evil, although his essence has been corrupted on open platform and he is banished to the unfathomable mainframe Abyss, but his power over terror left him incapable of feeling fear.', 60, 50, 150, 3);
INSERT INTO Boss VALUES ('MySQL','Was once the leader of opensource RDBMs Council, but he swore allegiance to Oracle later on. Much uncertainty surrounds him, his departure created a colossals fracture within the opensource council', 40, 40, 100, 2);
INSERT INTO Boss VALUES ('SQLServer','A son of great evil Microsoft, Lord of Windows realm, was so pervasive that even after he had been defeated by Linux angels in the enterprise territorial war', 40, 30, 100, 1);



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
