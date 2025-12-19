package main

import (
	"fmt"
	"log"
	"net"
	"netchatroom/netchat/Client/handClient"
	"netchatroom/netchat/common"
	"time"
)

func main() {
	//登入服务器
	conn, err := net.DialTimeout("tcp", "0.0.0.0:8888", 3*time.Second)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("连接超时...")
		} else {
			fmt.Printf("登入服务器失败 err:%v\n", err)
		}
		return
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			log.Printf("C.Conn.Close failed,err:%v\n", err)
		}
	}()

	C := &common.Client{Conn: conn}
	if handClient.LoginAndRegister(C) {
		go handClient.SendHeartbeat(C)
		handClient.HandleClient(C)
	}
}
