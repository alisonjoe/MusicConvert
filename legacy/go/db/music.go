package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db = new(sql.DB)

func Init() {
	//打开数据库，如果不存在，则创建
	var err error
	db, err = sql.Open("sqlite", "./musicConvert.db")
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return
	}
	if err := initTable(); err != nil {
		log.Printf("Failed to initialize database table: %v", err)
		return
	}
	log.Printf("db succ")
}

func initTable() error {
	sqlTable := `
			CREATE TABLE IF NOT EXISTS music_info (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					music TEXT UNIQUE NOT NULL,
				state INT NULL,
					created BIGINT NULL
			);
			`
	_, err := db.Exec(sqlTable)
	if err != nil {
		return fmt.Errorf("initTable error: %w", err)
	}
	return nil
}

type MusicInfo struct {
	Id      int
	Music   string
	State   int
	Created int64
}

func Insert(st *MusicInfo) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	stmt, err := db.Prepare("insert into music_info(music, state, created) values(?,?,?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(st.Music, st.State, time.Now().Unix())
	if err != nil {
		return err
	}
	log.Printf("insert succ! %#v", st)
	return nil
}

func Query(music string) ([]MusicInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := db.Query(`select id, music, state, created from music_info where music = ?`, music)
	if err != nil {
		log.Printf("query fail:%v", err)
		return nil, err
	}
	var l []MusicInfo
	for rows.Next() {
		var temp = MusicInfo{}
		err = rows.Scan(&temp.Id, &temp.Music, &temp.State, &temp.Created)
		if err != nil {
			log.Printf("row scan error: %v", err)
			continue
		}
		l = append(l, temp)
	}
	rows.Close()
	return l, nil
}

func DeleteByMusicPrefix(prefix string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return 0, fmt.Errorf("empty music prefix")
	}
	res, err := db.Exec(`delete from music_info where music = ? or music like ?`, prefix, prefix+"/%")
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	log.Printf("deleted %d conversion records under %s", rows, prefix)
	return rows, nil
}

func DeleteAll() (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	res, err := db.Exec(`delete from music_info`)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	log.Printf("deleted all conversion records: %d", rows)
	return rows, nil
}
