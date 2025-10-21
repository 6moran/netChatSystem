package main

import (
	"awesomeProject/day26/utils"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	UserName string
	Conn     net.Conn
}

type Message struct {
	SenderConn net.Conn // 发送者的地址
	Content    string   // 消息内容
}

var (
	cl             = sync.Mutex{}
	clients        = sync.Map{}              // 定义客户端map，用来存储不同客户端
	publicMsgChan  = make(chan Message, 100) // 公聊消息通道
	privateMsgChan = make(chan Message, 100) // 私聊消息通道
	joinChan       = make(chan *Client, 10)  // 用户加入通道
	leaveChan      = make(chan *Client, 10)  // 用户离开通道
)

// 用户名重复检测
func IsRepeat(conn net.Conn) bool {
	for {
		str, err := utils.ReadData(conn)
		if err != nil {
			fmt.Printf("[系统消息]：%s已关闭\n", conn.RemoteAddr().String())
			return false
		}
		if str == "/heartbeat" {
			continue
		}
		//加锁直到注册完毕
		cl.Lock()
		//找到重复的用户名
		isRepeat := false
		clients.Range(func(_, value interface{}) bool {
			client := value.(*Client)
			if client.UserName == str {
				isRepeat = true
				return false // 找到重复，停止遍历
			}
			return true
		})
		if !isRepeat {
			err := utils.WriteData(conn, "ok\n")
			if err != nil {
				fmt.Printf("用户名检测写入出错Errors：%v\n", err)
			}
			client := &Client{str, conn}
			//加入到加入消息通道中进行广播
			joinChan <- client
			//加入到map中用于后续的查看
			clients.Store(conn.RemoteAddr().String(), client)
			cl.Unlock()
			return true
		} else {
			err := utils.WriteData(conn, "用户名已存在，请重新输入\n")
			cl.Unlock()
			if err != nil {
				fmt.Printf("用户名检测写入出错Errors：%v\n", err)
			}
		}
	}
}

// 服务器广播
func Broadcast(senderConn net.Conn, str string) {
	//对除了发送者的所有用户发送
	clients.Range(func(_, value interface{}) bool {
		val, _ := value.(*Client)
		if val.Conn != senderConn {
			err := utils.WriteData(val.Conn, str)
			if err != nil {
				fmt.Printf("广播发送信息出现错误Errors：%v\n", err)
			}
		}
		return true
	})
}

// 接收消息并分类发送到相应管道
func receiveToChan(conn net.Conn) {
	defer conn.Close()
	for {
		msg, err := utils.ReadData(conn)
		c, _ := clients.Load(conn.RemoteAddr().String())
		if err != nil {
			leaveChan <- c.(*Client)
			return
		}
		switch {
		case msg == "/quit":
			c, _ := clients.Load(conn.RemoteAddr().String())
			leaveChan <- c.(*Client)
			return
		case msg == "/checkUser":
			//处理查看在线用户功能
			c, _ := clients.Load(conn.RemoteAddr().String())
			c.(*Client).checkUser()
		case strings.HasPrefix(msg, "/chatWith"):
			privateMsgChan <- Message{
				SenderConn: conn,
				Content:    msg,
			}
		case msg == "/heartbeat":
			er := utils.WriteData(conn, "/heartbeat\n")
			if er != nil {
				fmt.Println("服务器发送心跳出错", er)
			}
			continue
		default:
			publicMsgChan <- Message{
				SenderConn: conn,
				Content:    msg,
			}
		}
	}
}

// 处理查看在线用户功能
func (c *Client) checkUser() {
	list := ""
	num := 0
	clients.Range(func(_, value interface{}) bool {
		num++
		userName := value.(*Client).UserName
		list += userName + "   "
		return true // 继续遍历所有用户
	})

	endList := "当前在线用户(" + strconv.Itoa(num) + "人):" + list
	err := utils.WriteData(c.Conn, endList+"\n")
	if err != nil {
		fmt.Println("查看在线用户发送出错")
	}
	fmt.Printf("[系统消息]：%s请求查看了在线用户\n", c.UserName)
}

// 服务端的消息处理操作
func serviceSendMessage() {
	for {
		select {
		case msg := <-publicMsgChan:
			handlePublicMsg(msg)
		case joinClient := <-joinChan:
			joinClient.handleJoin()
		case leaveClient := <-leaveChan:
			leaveClient.handleLeave()
		case msg := <-privateMsgChan:
			handlePrivateMsg(msg)
		}
	}
}

// 处理客户端发送的私聊消息
func handlePrivateMsg(msg Message) {
	contend := msg.Content
	contend = strings.Replace(contend, "/chatWith", "", 1)
	sendToUser := make([]rune, 0)
	hasCloseBracket := false
	//得到私聊的客户端结构体
	c, _ := clients.Load(msg.SenderConn.RemoteAddr().String())
	if strings.HasPrefix(contend, "(") {
		for _, v := range contend {
			if v == ')' {
				hasCloseBracket = true
				break
			}
			sendToUser = append(sendToUser, v)
		}
	} else {
		err := utils.WriteData(msg.SenderConn, "私聊格式错误，请检查输入...\n")
		if err != nil {
			fmt.Println("私聊格式发送提示失败")
		}
		fmt.Printf("[系统消息]：%s请求私聊失败，因为格式错误\n", c.(*Client).UserName)
		return
	}
	if !hasCloseBracket {
		err := utils.WriteData(msg.SenderConn, "私聊格式错误，请检查输入...\n")
		if err != nil {
			fmt.Println("私聊格式发送提示失败")
		}
		fmt.Printf("[系统消息]：%s请求私聊失败，因为格式错误\n", c.(*Client).UserName)
		return
	} else {
		sendToUser = sendToUser[1:]
		endSendToUser := string(sendToUser)
		contend = strings.Replace(contend, "("+endSendToUser+")", "", 1)
		endMsg := fmt.Sprintf("%s私聊你：%s\n", c.(*Client).UserName, contend)
		//判断对自己发送私信的情况
		if endSendToUser == c.(*Client).UserName {
			err := utils.WriteData(msg.SenderConn, "不能对自己发送私聊...\n")
			fmt.Printf("[系统消息]：%s妄想对自己发送私聊...\n", endSendToUser)
			if err != nil {
				fmt.Println("私聊返回发送失败")
			}
			return
		}
		//true说明没找到对应的私聊对象
		flag := true
		//遍历得到对应的Conn，单独发送
		clients.Range(func(_, value interface{}) bool {
			if endSendToUser == value.(*Client).UserName {
				flag = false
				err := utils.WriteData(value.(*Client).Conn, endMsg)
				fmt.Printf("%s私聊%s：%s\n", c.(*Client).UserName, endSendToUser, contend)
				if err != nil {
					fmt.Println("私聊发送失败")
				}
				return false
			}
			return true
		})
		if flag {
			err := utils.WriteData(msg.SenderConn, "私聊对象不存在，请检查输入...\n")
			fmt.Printf("[系统消息]：%s请求私聊%s失败,因为该对象不存在...\n", c.(*Client).UserName, endSendToUser)
			if err != nil {
				fmt.Println("私聊返回发送失败")
			}
		}
	}
}

// 处理客户端发送的群聊消息
func handlePublicMsg(msg Message) {
	c, _ := clients.Load(msg.SenderConn.RemoteAddr().String())
	Broadcast(msg.SenderConn, fmt.Sprintf("->%s：%s\n", c.(*Client).UserName, msg.Content))
	fmt.Printf("->%s：%s\n", c.(*Client).UserName, msg.Content)
}

// 处理用户的加入消息
func (c *Client) handleJoin() {
	joinMsg := fmt.Sprintf("[系统消息]：%s加入了聊天室\n", c.UserName)
	Broadcast(c.Conn, joinMsg)
	fmt.Printf("[系统消息]：%s加入了聊天室\n", c.UserName)
}

// 处理用户的离开消息
func (c *Client) handleLeave() {
	clients.Delete(c.Conn.RemoteAddr().String())
	joinMsg := fmt.Sprintf("[系统消息]：%s离开了聊天室\n", c.UserName)
	Broadcast(c.Conn, joinMsg)
	fmt.Printf("[系统消息]：%s离开了聊天室\n", c.UserName)
}

func main() {
	fmt.Println("服务器启动...")
	listen, err := net.Listen("tcp", "0.0.0.0:8888")
	if err != nil {
		fmt.Println("服务器启动失败...,err=", err)
		return
	}
	fmt.Println("服务器启动成功...")
	//延时关闭listen
	defer listen.Close()
	go serviceSendMessage()
	for {
		//等待客户端链接
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("出现错误：", err)
			continue
		}
		//链接成功
		go func() {
			if IsRepeat(conn) {
				receiveToChan(conn)
			} else {
				fmt.Println("aa")
				conn.Close()
			}
		}()
	}
}
