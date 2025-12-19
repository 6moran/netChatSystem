package main

import (
	"fmt"
	"log"
	"net"
	"netchatroom/netchat/Service/handService"
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
		err = listen.Close()
		if err != nil {
			log.Printf("listen.Close failed,err:%v\n", err)
		}
	}()
	err = db.InitDB()
	if err != nil {
		log.Printf("InitDB failed,err:%v\n", err)
	}

	netchat := handService.Service{
		Clients: sync.Map{},
		MsgChan: make(chan *common.Message, 100),
	}

	go netchat.HandleMsgChan()
	for {
		//等待客户端链接
		conn, err := listen.Accept()
		if err != nil {
			log.Println("listen.Accept failed,err:", err)
			continue
		}
		//链接成功
		go func() {
			if C := netchat.LoginAndRegister(conn); C != nil {
				netchat.ReceiveToChan(C)
			}
		}()
	}
}
