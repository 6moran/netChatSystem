package common

import "net"

type Client struct {
	UserName string
	Conn     net.Conn `json:"-"`
}

type Message struct {
	Sender  *Client //发送者信息
	Content string  // 消息内容
	Type    int     // 消息类型
	To      string  // 私聊用
}
