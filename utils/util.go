package utils

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"time"
)

// 客户端和服务端之间的数据读取
func ReadData(conn net.Conn) (string, error) {
	//设置读超时（每次读前重置）
	er := conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if er != nil {
		return "", er
	}
	reader := bufio.NewReader(conn)
	str, err := reader.ReadString('\n')
	if err != nil {
		//net.Timeout()返回一个bool值，其判断该错误是否为超时错误
		//将错误类型断言为net.Error调用其下的net.Timeout函数
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", err
		}
		return "", err
	}
	return strings.Trim(str, "\r\n"), nil
}

func WriteData(conn net.Conn, str string) error {
	er := conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	if er != nil {
		return er
	}
	_, err := conn.Write([]byte(str))
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return errors.New("写数据超时")
		}
		return err
	}
	return nil
}
