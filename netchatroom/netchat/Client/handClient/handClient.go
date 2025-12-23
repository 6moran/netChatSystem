package handClient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"netchatroom/netchat/common"
	"netchatroom/netchat/message"
	"os"
	"strings"
	"time"
)

var (
	reader   = bufio.NewReader(os.Stdin)
	quitChan = make(chan struct{})
)

// 键盘输入函数
func KeyboardInput() (string, error) {
	msg, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("KeyboardInput failed,err:%v", err)
	}
	return strings.TrimSpace(msg), nil
}

// 登录/注册处理
func LoginAndRegister(C *common.Client) bool {
	for {
		select {
		case <-quitChan:
			return false
		default:
			fmt.Println("-------欢迎访问聊天室-------")
			fmt.Println("----------1.登录----------")
			fmt.Println("----------2.注册----------")
			fmt.Println("-------输入/quit退出-------")
			fmt.Println("请输入你要进行的操作:")
			n, err := KeyboardInput()
			if err != nil {
				log.Printf("LoginAndRegister KeyboardInput failed,err:%v\n", err)
			}
			switch n {
			case "1":
				if Login(C) {
					return true
				} else {
					continue
				}
			case "2":
				Register(C)
				continue
			case "/quit":
				return false
			}
		}
	}
}

func Register(C *common.Client) {
	for {
		fmt.Println("请输入用户名:")
		username, err := KeyboardInput()
		if err != nil {
			log.Printf("Register username failed,err:%v\n", err)
			continue
		}
		username = strings.TrimSpace(username)
		if username == "" {
			fmt.Println("用户名不允许为空")
			continue
		}
		if username == "/quit" {
			break
		}
		fmt.Println("请输入密码:")
		password, err := KeyboardInput()
		if err != nil {
			log.Printf("Register password failed,err:%v\n", err)
			continue
		}
		password = strings.TrimSpace(password)
		if password == "" {
			fmt.Println("密码不允许为空")
			continue
		}
		if password == "/quit" {
			break
		}
		err = message.SendMsg(C.Conn, &common.Message{
			Sender:  C,
			Content: username + "/" + password,
			Type:    message.Register,
		})
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				s := strings.ToLower(opErr.Err.Error())
				if strings.Contains(s, "forcibly closed") {
					fmt.Println("服务器已关闭...")
					close(quitChan)
					return
				}
			}
			log.Printf("Register SendMsg failed,err:%v\n", err)
			continue
		}

		reciveMsg, err := message.ReciveMsg(C.Conn)
		if err != nil {
			log.Printf("Register ReciveMsg failed ,err:%v\n", err)
			continue
		}
		if reciveMsg.Content == "ok" {
			fmt.Println("注册成功!")
			return
		} else {
			fmt.Println(reciveMsg.Content)
			return
		}
	}
}
func Login(C *common.Client) bool {
	for {
		fmt.Println("请输入用户名:")
		username, err := KeyboardInput()
		if err != nil {
			log.Printf("Login username failed,err:%v\n", err)
			continue
		}
		username = strings.TrimSpace(username)
		if username == "" {
			fmt.Println("用户名不允许为空")
			continue
		}
		if username == "/quit" {
			return false
		}
		fmt.Println("请输入密码:")
		password, err := KeyboardInput()
		if err != nil {
			log.Printf("Login password failed,err:%v\n", err)
			continue
		}
		password = strings.TrimSpace(password)
		if password == "" {
			fmt.Println("密码不允许为空")
			continue
		}
		if password == "/quit" {
			return false
		}
		err = message.SendMsg(C.Conn, &common.Message{
			Sender:  C,
			Content: username + "/" + password,
			Type:    message.Login,
		})
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				s := strings.ToLower(opErr.Err.Error())
				if strings.Contains(s, "forcibly closed") {
					fmt.Println("服务器已关闭...")
					close(quitChan)
					return false
				}
			}
			log.Printf("Login SendMsg failed,err:%v\n", err)
			continue
		}

		reciveMsg, err := message.ReciveMsg(C.Conn)
		if err != nil {
			log.Printf("Login ReciveMsg failed ,err:%v\n", err)
			continue
		}
		if reciveMsg.Content == "ok" {
			C.UserName = username
			fmt.Println("登录成功!")
			return true
		} else {
			fmt.Println(reciveMsg.Content)
			return false
		}
	}
}

// 用户循环接收服务端发送的信息
func ClientReceiveMsg(C *common.Client) {
	for {
		msg, err := message.ReciveMsg(C.Conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("您已心跳超时，服务端强制踢出...")
				close(quitChan)
				return
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				s := strings.ToLower(opErr.Err.Error())
				if strings.Contains(s, "forcibly closed") {
					fmt.Println("服务器已关闭...")
					close(quitChan)
					return
				}
			}
			log.Printf("ClientReceiveMsg ReciveMsg failed,err:%v\n", err)
			return
		}
		if msg.Type != message.HeartMsg {
			fmt.Println(msg.Content)
		}
	}
}

// 心跳检测
func SendHeartbeat(C *common.Client) {
	ticker := time.NewTicker(20 * time.Second) // 20秒发送一次
	defer ticker.Stop()
	for {
		select {
		//ticker结构体中有一个C通道
		case <-ticker.C:
			// 重置超时
			err := message.SendMsg(C.Conn, &common.Message{
				Sender: C,
				Type:   message.HeartMsg,
			})
			//发送失败就退出心跳，退出客户端
			if err != nil {
				log.Printf("SendHeartbeat SendMsg failed,err:%v\n", err)
				close(quitChan)
				return
			}
		case <-quitChan:
			return
		}
	}
}

// 登录后主进程
func HandleClient(C *common.Client) {
	fmt.Println("-------欢迎加入聊天室-------")
	fmt.Println("-----输入/help查看所有指令-----")
	go ClientReceiveMsg(C)

	inputChan := make(chan string, 10)
	go func() {
		for {
			input, err := KeyboardInput()
			if err != nil {
				log.Printf("HandleClient input failed,err:%v\n", err)
				continue
			}
			inputChan <- input
		}
	}()

	for {
		select {
		case <-quitChan:
			return
		case input := <-inputChan:
			switch {
			case input == "/help":
				fmt.Println("/quit--退出聊天室")
				fmt.Println("/checkUser--查看在线用户")
				fmt.Println("/chat 用户名:消息--私聊用户")
			case input == "/quit":
				err := message.SendMsg(C.Conn, &common.Message{
					Sender: C,
					Type:   message.Quit,
				})
				if err != nil {
					log.Printf("HandleClient sendMsg quit failed,err:%v\n", err)
				}
				close(quitChan)
				return
			case input == "/checkUser":
				err := message.SendMsg(C.Conn, &common.Message{
					Sender: C,
					Type:   message.CheckUser,
				})
				if err != nil {
					log.Printf("HandleClient sendMsg checkUser failed,err:%v\n", err)
				}
			case strings.HasPrefix(input, "/chat"):
				result := strings.SplitN(input, " ", 2)
				if len(result) != 2 {
					fmt.Println("私聊格式错误，请重新输入...")
					continue
				}
				result2 := strings.SplitN(result[1], ":", 2)
				if len(result2) != 2 {
					fmt.Println("私聊格式错误，请重新输入...")
					continue
				}
				if C.UserName == result2[0] {
					fmt.Println("不能对自己私聊...")
					continue
				}
				//发送给服务端
				err := message.SendMsg(C.Conn, &common.Message{
					Sender:  C,
					Type:    message.PrivateMsg,
					Content: result2[1],
					To:      result2[0],
				})
				if err != nil {
					log.Printf("HandleClient sendMsg chat failed,err:%v\n", err)
				}
			case strings.HasPrefix(input, "/"):
				fmt.Println("指令输入错误，请检查输入...")
			case input == "":
				fmt.Println("输入内容不能为空...")
			default:
				err := message.SendMsg(C.Conn, &common.Message{
					Sender:  C,
					Type:    message.PublicMsg,
					Content: input,
				})
				if err != nil {
					log.Printf("HandleClient SendMsg input failed,err:%v\n", err)
				}
			}

		}
	}
}
