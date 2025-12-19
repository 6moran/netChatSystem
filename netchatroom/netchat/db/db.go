package db

import (
	"fmt"
	"github.com/jmoiron/sqlx"
)

var db *sqlx.DB

//type user struct {
//	id       int
//	username string
//	password string
//}

// 初始化连接数据库
func InitDB() (err error) {
	//数据库信息
	dsn := "root:1458963@tcp(127.0.0.1:3306)/netchat"
	//连接数据库
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		return fmt.Errorf("Connect failed ,err:%w", err)
	}
	return nil
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
