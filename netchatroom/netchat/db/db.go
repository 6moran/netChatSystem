package db

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"log"
	"time"
)

var db *sqlx.DB

//type user struct {
//	id       int
//	username string
//	password string
//}

// 初始化连接数据库
func InitDB() (err error) {
	//go项目数据库信息
	//dsn := "root:1458963@tcp(127.0.0.1:3306)/netchat"
	//部署docker后数据库信息
	dsn := "netchat:netchat@tcp(mysql:3306)/netchat"
	//连接数据库
	for i := 1; i <= 10; i++ {
		fmt.Printf("第%v次连接数据库\n", i)
		db, err = sqlx.Connect("mysql", dsn)
		if err == nil {
			fmt.Println("连接数据库成功!")
			return
		}
		time.Sleep(5 * time.Second)
	}
	err = fmt.Errorf("InitDB sqlx.Connect failed,err:%w", err)
	return
}

// 查询username是否存在
func QueryUsername(username string) (string, error) {
	sqlStr := "select password from user where username = ?"
	var password string
	err := db.Get(&password, sqlStr, username)
	if err != nil {
		return "", fmt.Errorf("Get failed,err:%w", err)
	}
	return password, nil
}

// 将user加入数据库
func AddUser(username string, password string) error {
	sqlStr := "insert into user(username,password) values(?,?)"
	_, err := db.Exec(sqlStr, username, password)
	if err != nil {
		return fmt.Errorf("Exec failed,err:%w", err)
	}
	return nil
}

func CloseDB() {
	err := db.Close()
	if err != nil {
		log.Printf("db.Close failed,err:%v\n", err)
	}
}
