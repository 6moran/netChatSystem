package handService

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net"
	"netchatroom/netchat/common"
	"netchatroom/netchat/db"
	"netchatroom/netchat/message"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Service struct {
	Clients sync.Map             // 用来存储在线客户端
	MsgChan chan *common.Message // 消息通道
}

// 服务器广播
func (S *Service) Broadcast(conn net.Conn, msg *common.Message) {
	//对除了发送者的所有用户发送
	S.Clients.Range(func(_, value interface{}) bool {
		val, _ := value.(*common.Client)
		if val.Conn != conn {
			err := message.SendMsg(val.Conn, msg)
			if err != nil {
				log.Printf("Broadcast SendMsg failed,err:%v\n", err)
			}
		}
		return true
	})
}

// 接收消息
func (S *Service) ReceiveToChan(C *common.Client) {
	for {
		msg, err := message.ReciveMsg(C.Conn)
		if err != nil {
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				s := strings.ToLower(opErr.Err.Error())
				if strings.Contains(s, "forcibly closed") {
					S.MsgChan <- &common.Message{
						Sender: C,
						Type:   message.Quit,
					}
					return
				}
			}
			//net.Timeout()返回一个bool值，其判断该错误是否为超时错误
			//将错误类型断言为net.Error调用其下的net.Timeout函数
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() {
					S.MsgChan <- &common.Message{
						Content: fmt.Sprintf("%v心跳超时，强行踢出!", C.UserName),
					}
					S.MsgChan <- &common.Message{
						Sender: C,
						Type:   message.Quit,
					}
					return
				}
			}
			log.Printf("ReceiveToChan ReciveMsg failed,err:%v\n", err)
			return

		}
		msg.Sender.Conn = C.Conn
		S.MsgChan <- msg
	}
}

// 处理管道中的消息进行相应操作
func (S *Service) HandleMsgChan() {
	for {
		msg := <-S.MsgChan
		switch msg.Type {
		case message.Join:
			S.handleJoin(msg.Sender)
		case message.Quit:
			S.handleLeave(msg.Sender)
		case message.PublicMsg:
			S.Broadcast(msg.Sender.Conn, &common.Message{
				Content: fmt.Sprintf("->%v:%v", msg.Sender.UserName, msg.Content),
			})
			fmt.Printf("->%v:%v\n", msg.Sender.UserName, msg.Content)
		case message.PrivateMsg:
			S.handlePrivateMsg(msg)
		case message.HeartMsg:
			err := msg.Sender.Conn.SetReadDeadline(time.Now().Add(50 * time.Second))
			if err != nil {
				log.Printf("HandleMsgChan SetReadDeadline failed,err:%v\n", err)
			}
			continue
		case message.CheckUser:
			S.handleCheckUser(msg.Sender)
		default:
			fmt.Printf("[系统消息]:%v\n", msg.Content)
		}
	}

}

// 处理私聊的消息
func (S *Service) handlePrivateMsg(msg *common.Message) {
	reciveC, ok := S.Clients.Load(msg.To)
	if ok == false {
		err := message.SendMsg(msg.Sender.Conn, &common.Message{
			Content: fmt.Sprintf("该用户(%v)不在线", msg.To),
		})
		if err != nil {
			log.Printf("hanlePrivateMsg SendMsg1 failed,err:%v\n", err)
			return
		}
		return
	}
	err := message.SendMsg(reciveC.(*common.Client).Conn, &common.Message{
		Content: fmt.Sprintf("%v私聊你:%v", msg.Sender.UserName, msg.Content),
	})
	if err != nil {
		log.Printf("hanlePrivateMsg SendMsg2 failed,err:%v\n", err)
		return
	}
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v私聊%v:%v", msg.Sender.UserName, reciveC.(*common.Client).UserName, msg.Content),
	}

}

// 处理用户的加入消息
func (S *Service) handleJoin(C *common.Client) {
	S.Clients.Store(C.UserName, C)
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v加入聊天室!", C.UserName),
	}
	S.Broadcast(C.Conn, &common.Message{
		Content: fmt.Sprintf("[系统消息]:%v加入聊天室", C.UserName),
	})
}

// 处理用户的离开消息
func (S *Service) handleLeave(C *common.Client) {
	S.Clients.Delete(C.UserName)
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v离开了聊天室!", C.UserName),
	}
	S.Broadcast(C.Conn, &common.Message{
		Content: fmt.Sprintf("[系统消息]:%v离开了聊天室!", C.UserName),
	})
	//做完退出操作后关闭Conn
	err := C.Conn.Close()
	if err != nil {
		log.Printf("C.Conn.Close failed,err:%v\n", err)
	}
}

// 处理查看在线用户功能
func (S *Service) handleCheckUser(C *common.Client) {
	list := ""
	num := 0
	S.Clients.Range(func(_, value interface{}) bool {
		num++
		userName := value.(*common.Client).UserName
		list += userName + "   "
		return true // 继续遍历所有用户
	})
	endList := "当前在线用户(" + strconv.Itoa(num) + "人):" + list
	err := message.SendMsg(C.Conn, &common.Message{
		Content: endList,
	})
	if err != nil {
		log.Printf("CheckUser SendMsg endList failed,err:%v\n", err)
	}
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%s请求查看了在线用户", C.UserName),
	}
}

// 对登录注册消息进行区别和处理
func (S *Service) LoginAndRegister(conn net.Conn) *common.Client {
	for {
		msg, err := message.ReciveMsg(conn)
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				s := strings.ToLower(opErr.Err.Error())
				if strings.Contains(s, "forcibly closed") {
					return nil
				}
			}
			log.Printf("LoginAndRegister ReciveMsg failed,err:%v\n", err)
			return nil
		}
		msg.Sender.Conn = conn
		switch msg.Type {
		case message.Register:
			S.ReplyRegister(msg)
			continue
		case message.Login:
			if C := S.ReplyLogin(msg); C != nil {
				return C
			} else {
				continue
			}
		}
	}
}

// 用户注册消息回复
func (S *Service) ReplyRegister(msg *common.Message) {
	//if str == "/heartbeat" {
	//	continue
	//}
	//加锁直到注册完毕
	//S.cl.Lock()
	user := strings.Split(msg.Content, "/")
	//加入数据库中
	err := db.AddUser(user[0], user[1])
	if err != nil {
		//判断用户名是否存在，这里有唯一约束会添加失败
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名已存在，请登录",
			})
			if er != nil {
				log.Printf("ReplyRegister AddUser SendMsg2 failed,err:%v\n", er)
			}
		} else {
			log.Printf("ReplyRegister AddUser failed,err:%v\n", err)
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "注册失败，请稍后再试",
			})
			if er != nil {
				log.Printf("ReplyRegister AddUser SendMsg3 failed,err:%v\n", er)
			}
		}
		return
	}
	//注册成功
	err = message.SendMsg(msg.Sender.Conn, &common.Message{
		Content: "ok",
	})
	if err != nil {
		log.Printf("ReplyRegister AddUser SendMsg4 failed,err:%v\n", err)
	}
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v注册成功!", user[0]),
	}
}

func (S *Service) ReplyLogin(msg *common.Message) *common.Client {
	user := strings.Split(msg.Content, "/")
	password, err := db.QueryUsername(user[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名不存在，请注册",
			})
			if er != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg2 failed,err:%v\n", er)
			}
		} else {
			log.Printf("ReplyLogin QueryUsername failed,err:%v\n", err)
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "登录失败，请稍后再试",
			})
			if er != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg3 failed,err:%v\n", er)
			}
		}
		return nil
	}
	if user[1] == password {
		if _, ok := S.Clients.Load(user[0]); ok {
			err = message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名已登录...",
			})
			if err != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg6 failed,err:%v\n", err)
			}
		} else {
			//登录成功
			err = message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "ok",
			})
			if err != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg4 failed,err:%v\n", err)
			}
			client := &common.Client{user[0], msg.Sender.Conn}
			//加入到map中用于后续的查看
			S.MsgChan <- &common.Message{
				Sender:  client,
				Content: fmt.Sprintf("%v加入聊天室!\n", user[0]),
				Type:    message.Join,
			}
			//从登录成功起开始接收心跳，设置心跳超时时间
			err = msg.Sender.Conn.SetReadDeadline(time.Now().Add(50 * time.Second))
			if err != nil {
				log.Printf("HandleMsgChan SetReadDeadline failed,err:%v\n", err)
			}
			return client
		}
	} else {
		err = message.SendMsg(msg.Sender.Conn, &common.Message{
			Content: "密码错误，请重新输入",
		})
		if err != nil {
			log.Printf("ReplyLogin QueryUsername SendMsg5 failed,err:%v\n", err)
		}
		return nil
	}
	return nil
}
