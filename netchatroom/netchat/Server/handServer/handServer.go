package handServer

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
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

type Server struct {
	Clients sync.Map             // 用来存储在线客户端
	MsgChan chan *common.Message // 消息通道
}

// Broadcast 服务器广播
func (S *Server) Broadcast(username string, msg *common.Message) {
	//对除了发送者的所有在线用户发送
	S.Clients.Range(func(_, value interface{}) bool {
		val, _ := value.(*common.Client)
		if val.UserName != username {
			err := message.SendMsg(val.Conn, msg)
			if err != nil {
				log.Printf("Broadcast SendMsg failed,err:%v\n", err)
			}
		}
		return true
	})
}

// ReceiveToChan 接收消息
func (S *Server) ReceiveToChan(C *common.Client) {
	for {
		msg, err := message.ReciveMsg(C.Conn)
		if err != nil {
			//客户端主动退出
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			//客户端异常退出
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

// HandleUsernameStreamMsg 处理私聊流中的消息
func (S *Server) HandleUsernameStreamMsg(C *common.Client) {
	for {
		msgID, msgStr, err := db.XReadGroupMsg(C.UserName+"_stream", C.UserName+"_group", C.UserName+"_consumer")
		if err != nil {
			log.Printf("HandleUsernameStreamMsg db.XReadGroupMsg failed,err:%v\n", err)
			continue
		}
		msg, err := message.JsonToMsg(msgStr)
		if err != nil {
			log.Printf("HandleUsernameStreamMsg JsonToMsg failed,err:%v\n", err)
			continue
		}
		if msg.Sender.UserName == "[退出信号]" {
			//直接确认
			err = db.XAckMsg(msgID, C.UserName+"_stream", C.UserName+"_group")
			if err != nil {
				log.Printf("HandleUsernameStreamMsg db.XAckMsg failed,err:%v\n", err)
			}
			return
		}
		receiveC, _ := S.Clients.Load(C.UserName)
		err = message.SendMsg(receiveC.(*common.Client).Conn, &common.Message{
			Content: fmt.Sprintf("->%v私聊你:%v", msg.Sender.UserName, msg.Content),
		})
		if err != nil {
			log.Printf("HandleUsernameStreamMsg SendMsg failed,err:%v\n", err)
			return
		}

		//发送完确认
		err = db.XAckMsg(msgID, C.UserName+"_stream", C.UserName+"_group")
		if err != nil {
			log.Printf("HandleUsernameStreamMsg db.XAckMsg failed,err:%v\n", err)
			continue
		}
	}
}

// HandleMsgStream 处理群聊流中的消息
func (S *Server) HandleMsgStream() {
	for {
		msgID, msgStr, err := db.XReadGroupMsg(db.ReceiveStreamName, db.GroupName, db.ConsumerName)
		if err != nil {
			log.Printf("HandleMsgStream db.XReadGroupMsg failed,err:%v\n", err)
			continue
		}
		msg, err := message.JsonToMsg(msgStr)
		if err != nil {
			log.Printf("HandleMsgStream JsonToMsg failed,err:%v\n", err)
			continue
		}
		switch msg.Type {
		case message.PublicMsg:
			S.HandlePublicMsg(msg, msgID)
		default:
			continue
			//err = db.XAckMsg(msgID, db.ReceiveStreamName, db.GroupName)
			//if err != nil {
			//	log.Printf("HandleMsgStream db.XAckMsg failed,err%v", err)
			//}
		}
	}
}

// HandleMsgChan 处理管道中的消息进行相应操作，进不进流
func (S *Server) HandleMsgChan() {
	for {
		msg := <-S.MsgChan
		switch msg.Type {
		case message.Join:
			S.HandleJoin(msg.Sender)
		case message.Quit:
			S.HandleLeave(msg.Sender)
		case message.PublicMsg:
			rdbMsg, err := message.MsgToJson(msg)
			if err != nil {
				log.Printf("HandleMsgChan message.MsgToJson1 failed,err:%v\n", err)
			}
			//加到接收消息
			err = db.XAddMsg(rdbMsg, db.ReceiveStreamName)
			if err != nil {
				log.Printf("HandleMsgChan db.XAddMsg1 failed,err:%v\n", err)
			}
		case message.PrivateMsg:
			S.HandlePrivateMsg(msg)
		case message.HeartMsg:
			err := msg.Sender.Conn.SetReadDeadline(time.Now().Add(50 * time.Second))
			if err != nil {
				log.Printf("HandleMsgChan SetReadDeadline failed,err:%v\n", err)
			}
		case message.CheckUser:
			S.HandleCheckUser(msg.Sender)
		case message.CheckRankList:
			S.HandleCheckRankList(msg.Sender)
		case message.PublicHistory:
			S.HandlePublicHistory(msg)
		case message.PrivateHistory:
			S.HandlePrivateHistory(msg)
		default:
			fmt.Printf("[系统消息]%v\n", msg.Content)
		}
	}

}

// HandlePublicHistory 处理群聊历史消息
func (S *Server) HandlePublicHistory(msg *common.Message) {
	n, err := strconv.Atoi(msg.Content)
	if err != nil {
		log.Printf("HandlePublicHistory strconv.Atoi failed,err:%v\n", err)
		return
	}
	res, err := db.XRangeMsg(db.ReceiveStreamName, n)
	if err != nil {
		log.Printf("HandlePublicHistory XRangeMsg failed,err:%v", err)
		return
	}

	list := ""
	for _, v := range res {
		his, err := message.JsonToMsg(v)
		if err != nil {
			log.Printf("HandlePublicHistory JsonToMsg failed,err:%v", err)
			return
		}
		list = list + "->" + his.Sender.UserName + ":" + his.Content + "\n"
	}
	err = message.SendMsg(msg.Sender.Conn, &common.Message{
		Type:    message.PublicHistory,
		Content: list,
	})
	fmt.Printf("[系统消息]%s请求查看了群聊历史消息\n", msg.Sender.UserName)
}

// HandlePrivateHistory 处理私聊历史消息
func (S *Server) HandlePrivateHistory(msg *common.Message) {
	n, err := strconv.Atoi(msg.Content)
	if err != nil {
		log.Printf("HandlePrivateHistory strconv.Atoi failed,err:%v\n", err)
		return
	}
	//判断该用户是否存在
	_, err = db.QueryUsername(msg.To)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名不存在，请检查输入",
			})
			if er != nil {
				log.Printf("HandlePrivateHistory QueryUsername SendMsg failed,err:%v\n", er)
			}
		}
		return
	}
	var streamName string
	if msg.Sender.UserName > msg.To {
		streamName = msg.Sender.UserName + "And" + msg.To
	} else {
		streamName = msg.To + "And" + msg.Sender.UserName
	}
	res, err := db.XRangeMsg(streamName, n)
	if err != nil {
		log.Printf("HandlePrivateHistory db.XRangeMsg failed,err:%v\n", err)
	}
	list := ""
	for _, v := range res {
		list = list + v + "\n"
	}
	err = message.SendMsg(msg.Sender.Conn, &common.Message{
		Type:    message.PrivateHistory,
		Content: list,
	})
	fmt.Printf("[系统消息]%s请求查看了与%s的私聊历史消息\n", msg.Sender.UserName, msg.To)
}

// HandlePublicMsg 处理流中公聊的消息
func (S *Server) HandlePublicMsg(msg *common.Message, msgID string) {
	defer func() {
		err := db.XAckMsg(msgID, db.ReceiveStreamName, db.GroupName)
		if err != nil {
			log.Printf("HandlePublicMsg db.XAckMsg failed,err:%v", err)
			return
		}
	}()
	S.Broadcast(msg.Sender.UserName, &common.Message{
		Content: fmt.Sprintf("->%v:%v", msg.Sender.UserName, msg.Content),
	})
	fmt.Printf("->%v:%v\n", msg.Sender.UserName, msg.Content)
	//用户公聊消息触发添加活跃度
	err := db.ZIncrMsg(msg.Sender.UserName, db.ZSetName)
	if err != nil {
		log.Printf("ReceiveToChan ReceiveMsg failed,err:%v\n", err)
	}
}

// HandlePrivateMsg 处理私聊的消息
func (S *Server) HandlePrivateMsg(msg *common.Message) {
	//判断该用户是否存在
	_, err := db.QueryUsername(msg.To)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			er := message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名不存在，请检查输入",
			})
			if er != nil {
				log.Printf("HandlePrivateMsg QueryUsername SendMsg failed,err:%v\n", er)
			}
		}
		return
	}
	//直接发送到To用户的私聊收件箱中
	rdbMsg, err := message.MsgToJson(msg)
	if err != nil {
		log.Printf("HanlePrivateMsg message.MsgToJson failed,err:%v\n", err)
		return
	}
	//加到接收消息
	err = db.XAddMsg(rdbMsg, msg.To+"_stream")
	if err != nil {
		log.Printf("HanlePrivateMsg db.XAddMsg failed,err:%v\n", err)
		return
	}
	fmt.Printf("[系统消息]%v私聊%v:%v\n", msg.Sender.UserName, msg.To, msg.Content)
	//运用比较来统一key值，确保同一个流
	var streamName string
	if msg.Sender.UserName > msg.To {
		streamName = msg.Sender.UserName + "And" + msg.To
	} else {
		streamName = msg.To + "And" + msg.Sender.UserName
	}
	rdbMMsg := fmt.Sprintf("[私聊]%v:%v", msg.Sender.UserName, msg.Content)
	//加入特定的私聊历史消息流
	err = db.XAddMsg(rdbMMsg, streamName)
	if err != nil {
		log.Printf("HandleMsgChan db.XAddMsg4 failed,err:%v\n", err)
	}
	//用户私聊消息触发添加活跃度
	err = db.ZIncrMsg(msg.Sender.UserName, db.ZSetName)
	if err != nil {
		log.Printf("ReceiveToChan ReceiveMsg failed,err:%v\n", err)
	}
}

// HandleJoin 处理用户的加入消息
func (S *Server) HandleJoin(C *common.Client) {
	S.Clients.Store(C.UserName, C)
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v加入聊天室!", C.UserName),
	}
	S.Broadcast(C.UserName, &common.Message{
		Content: fmt.Sprintf("[系统消息]%v加入聊天室", C.UserName),
	})
}

// HandleLeave 处理用户的离开消息
func (S *Server) HandleLeave(C *common.Client) {
	S.Clients.Delete(C.UserName)
	S.MsgChan <- &common.Message{
		Content: fmt.Sprintf("%v离开了聊天室!", C.UserName),
	}
	S.Broadcast(C.UserName, &common.Message{
		Content: fmt.Sprintf("[系统消息]%v离开了聊天室!", C.UserName),
	})
	//做完退出操作后关闭Conn
	err := C.Conn.Close()
	if err != nil {
		log.Printf("C.Conn.Close failed,err:%v\n", err)
	}
	//写一个退出信号关闭单独的私聊协程
	err = db.XAddMsg("{\"Sender\":{\"UserName\":\"[退出信号]\"},\"Type\":5,\"To\":\"wuhan\"}", C.UserName+"_stream")
	if err != nil {
		log.Printf("HandleLeave XAddMsg [退出信号] failed,err:%v\n", err)
		return
	}
}

// HandleCheckRankList 处理用户查看活跃度排行榜功能
func (S *Server) HandleCheckRankList(C *common.Client) {
	lists, err := db.ZRevRangeMsg(db.ZSetName)
	if err != nil {
		log.Printf("HandleCheckRankList failed,err:%v\n", err)
		return
	}
	res := "-------" + "活跃度排行榜" + "-------\n"
	res = res + fmt.Sprintf("%-6s%-7s%-6s\n", "排名", "用户名", "活跃度")
	for i, list := range lists {
		res = res + fmt.Sprintf("%-7d%-10s%-6.0f\n", i+1, list.Member, list.Score)
	}
	res = res + "------------------------" + "\n"
	err = message.SendMsg(C.Conn, &common.Message{
		Type:    message.CheckRankList,
		Content: res,
	})
	if err != nil {
		log.Printf("HandleCheckRankList SendMsg res failed,err:%v\n", err)
	}
	fmt.Printf("[系统消息]%s请求查看了活跃度排行榜\n", C.UserName)
}

// HandleCheckUser 处理查看在线用户功能
func (S *Server) HandleCheckUser(C *common.Client) {
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
		log.Printf("HandleCheckUser SendMsg endList failed,err:%v\n", err)
	}
	fmt.Printf("[系统消息]%s请求查看了在线用户\n", C.UserName)
}

// LoginAndRegister 对登录注册消息进行区别和处理
func (S *Server) LoginAndRegister(conn net.Conn) *common.Client {
	for {
		msg, err := message.ReciveMsg(conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
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

// ReplyRegister 用户注册消息回复
func (S *Server) ReplyRegister(msg *common.Message) {
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

// ReplyLogin 用户登录消息回复
func (S *Server) ReplyLogin(msg *common.Message) *common.Client {
	user := strings.Split(msg.Content, "/")
	password, err := db.GetUser(user[0])
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Printf("ReplyLogin GetUser failed,err:%v\n", err)
		}
		//报错后继续查数据库，如果都查不到就返回
		password, err = db.QueryUsername(user[0])
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
		err = db.SetUser(user[0], password)
		if err != nil {
			log.Printf("ReplyLogin SetUser failed,err:%v", err)
		}
	}
	if user[1] == password {
		if _, ok := S.Clients.Load(user[0]); ok {
			err = message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "该用户名已登录...",
			})
			if err != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg6 failed,err:%v\n", err)
			}
			return nil
		} else {
			//登录成功
			err = message.SendMsg(msg.Sender.Conn, &common.Message{
				Content: "ok",
			})
			if err != nil {
				log.Printf("ReplyLogin QueryUsername SendMsg4 failed,err:%v\n", err)
			}
			client := &common.Client{UserName: user[0], Conn: msg.Sender.Conn}
			//加入到map中用于后续的查看
			S.MsgChan <- &common.Message{
				Sender:  client,
				Content: fmt.Sprintf("%v加入聊天室!\n", user[0]),
				Type:    message.Join,
			}
			//从登录成功起开始接收心跳，设置心跳超时时间
			err = msg.Sender.Conn.SetReadDeadline(time.Now().Add(50 * time.Second))
			if err != nil {
				log.Printf("ReplyLogin SetReadDeadline failed,err:%v\n", err)
				return nil
			}
			//为首次登录的用户创建用户组和流作为私聊收件箱
			err = db.XGroupCreateMkStreamMsg(user[0]+"_stream", user[0]+"_group")
			if err != nil {
				log.Printf("ReplyLogin XGroupCreateMkStreamMsg failed,err:%v", err)
				return nil
			}
			//为登录的用户创建或添加活跃度
			flag, err := db.ZAddNXMsg(user[0], db.ZSetName)
			if err != nil {
				log.Printf("ReplyLogin ZAddNXMsg failed,err:%v", err)
				return nil
			}
			//如果已经有了直接添加活跃度
			if flag == 0 {
				err = db.ZIncrMsg(user[0], db.ZSetName)
				if err != nil {
					log.Printf("ReplyLogin ZIncrMsg failed,err:%v", err)
					return nil
				}
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
}
