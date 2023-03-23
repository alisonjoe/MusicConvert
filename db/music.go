package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db = new(sql.DB)

func Init() {
	//打开数据库，如果不存在，则创建
	var err error
	db, err = sql.Open("sqlite3", "./musicConvert.db")
	if err != nil {
		panic(err)
	}
	if err := initTable(); err != nil {
		panic(err)
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
		panic(err)
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
	rows, err := db.Query(fmt.Sprintf(`select * from music_info where music = "%s"`, music))
	if err != nil {
		log.Fatalf("query fail:%v", err)
		return nil, err
	}
	var l []MusicInfo
	for rows.Next() {
		var temp = MusicInfo{}
		err = rows.Scan(&temp.Id, &temp.Music, &temp.State, &temp.Created)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		l = append(l, temp)
	}
	rows.Close()
	return l, nil
}
