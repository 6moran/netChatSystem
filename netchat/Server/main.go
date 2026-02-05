package main

import (
	"fmt"
	"log"
	"net"
	"netchatroom/netchat/Server/handServer"
	"netchatroom/netchat/common"
	"netchatroom/netchat/db"
	"sync"
)

func main() {
	fmt.Println("服务器启动...")
	listen, err := net.Listen("tcp", "0.0.0.0:8888")
	if err != nil {
		log.Println("服务器启动失败...,err:", err)
		return
	}
	fmt.Println("服务器启动成功...")
	//延时关闭listen
	defer func() {
		listenErr := listen.Close()
		if listenErr != nil {
			log.Printf("listen.Close failed,err:%v\n", listenErr)
		}
		db.CloseDB()
		db.CloseRDB()
	}()
	err = db.InitDB()
	if err != nil {
		log.Printf("InitDB failed,err:%v\n", err)
		return
	}
	err = db.InitRDB()
	if err != nil {
		log.Printf("InitRDB failed,err:%v\n", err)
		return
	}

	netChat := handServer.Server{
		Clients: sync.Map{},
		MsgChan: make(chan *common.Message, 100),
	}

	go netChat.HandleMsgChan()
	go netChat.HandleMsgStream()
	for {
		//等待客户端链接
		conn, err := listen.Accept()
		if err != nil {
			log.Println("listen.Accept failed,err:", err)
			continue
		}
		//链接成功
		go func() {
			if C := netChat.LoginAndRegister(conn); C != nil {
				go netChat.ReceiveToChan(C)
				go netChat.HandleUsernameStreamMsg(C)
			}
		}()
	}
}
