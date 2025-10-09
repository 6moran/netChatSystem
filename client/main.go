package main

import (
	"awesomeProject/day26/utils"
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// 用户名检测
func inPutUserName(conn net.Conn) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("请输入用户名：")
		username, err := reader.ReadString('\n')
		username = strings.TrimSpace(username) + "\n"
		if err != nil {
			fmt.Println("读取用户名失败")
			return false
		}
		if username == "\n" {
			fmt.Println("用户名为空，请重新输入")
			continue
		} else {
			err := utils.WriteData(conn, username)
			if err != nil {
				fmt.Printf("用户名写入出现Error：%v\n", err)
				return false
			}
		}

		//读取服务端返回来的值
		str, err1 := utils.ReadData(conn)
		if err1 != nil {
			fmt.Printf("用户读取出现Errors：%v\n", err1)
		}
		if strings.Trim(str, "\n") == "ok" {
			return true
		}
		fmt.Println(str)
	}
}

// 用户循环接收服务端发送的信息
func ReceiveMsg(conn net.Conn, quitChan chan struct{}) {
	for {
		msg, err := utils.ReadData(conn)
		if err != nil {
			select {
			case <-quitChan:
				return
			default:
				fmt.Printf("服务端已关闭\n")
				close(quitChan)
				return
			}
		}
		if msg != "/heartbeat" {
			fmt.Println(msg)
		}
	}
}

// 处理客户端的主要功能
func handleClient(conn net.Conn) {
	var flagChan = make(chan struct{})
	go sendHeartbeat(conn, flagChan)
	if inPutUserName(conn) {
		fmt.Println("-------欢迎加入聊天室-------")
		fmt.Println("-----输入/help查看所有指令-----")
		go ReceiveMsg(conn, flagChan)

		inputChan := make(chan string, 10)
		go func() {
			reader := bufio.NewReader(os.Stdin)
			for {
				str, err := reader.ReadString('\n')
				//键盘录入的\n实际字符串中通包含两个独立字符\和n
				if err != nil {
					fmt.Println("客户输入读取失败...")
					continue
				}
				inputChan <- str
			}
		}()
		for {
			select {
			case <-flagChan:
				return
			case str := <-inputChan:
				switch {
				case strings.Trim(str, "\r\n") == "/help":
					fmt.Println("/quit--退出聊天室")
					fmt.Println("/checkUser--查看在线用户")
					fmt.Println("/chatWith(用户名)消息--私聊用户")
				case strings.Trim(str, "\r\n") == "/quit":
					fallthrough
				case strings.Trim(str, "\r\n") == "/checkUser":
					fallthrough
				case strings.HasPrefix(strings.Trim(str, "\r\n"), "/chatWith"):
					//发送给服务端
					err2 := utils.WriteData(conn, str)
					if err2 != nil {
						fmt.Println("客户发送失败...")
					}
					if strings.Trim(str, "\r\n") == "/quit" {
						close(flagChan)
						fmt.Println("成功退出聊天室...")
						return
					}
				case strings.HasPrefix(strings.Trim(str, "\r\n"), "/"):
					fmt.Println("指令输入错误，请检查输入...")
				case strings.TrimSpace(str) == "":
					fmt.Println("输入内容不能为空...")
				default:
					err2 := utils.WriteData(conn, str)
					if err2 != nil {
						fmt.Println("客户发送失败...")
					}

				}

			}
		}

	}
}

// 客户端心跳函数：定期发送空消息（或 "/heartbeat" 指令）
func sendHeartbeat(conn net.Conn, quitChan chan struct{}) {
	ticker := time.NewTicker(15 * time.Second) // 15秒发送一次
	defer ticker.Stop()
	for {
		select {
		//ticker结构体中有一个C通道
		case <-ticker.C:
			// 发送/heartbeat用于重置超时
			err := utils.WriteData(conn, "/heartbeat\n")
			//发送失败就退出心跳，退出客户端
			if err != nil {
				return
			}
		case <-quitChan:
			return
		}
	}
}

func main() {
	//登入服务器
	conn, err := net.DialTimeout("tcp", "0.0.0.0:8888", 3*time.Second)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("连接超时...")
		} else {
			fmt.Printf("登入服务器失败 err=%v\n", err)
		}
		return
	}
	fmt.Println("登入服务器成功")
	defer conn.Close()
	handleClient(conn)
}
