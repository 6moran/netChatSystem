package utils

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// 客户端和服务端之间的数据读取
func ReadData(conn net.Conn) (string, error) {
	////设置读超时（每次读前重置）
	//err := conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	//if err != nil {
	//	return "", err
	//}

	var length uint32
	//读取消息长度
	err := binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return "", err
	}

	message := make([]byte, length)
	_, err = io.ReadFull(conn, message)
	if err != nil {
		////net.Timeout()返回一个bool值，其判断该错误是否为超时错误
		////将错误类型断言为net.Error调用其下的net.Timeout函数
		//if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		//	return "", errors.New("读数据超时")
		//}
		return "", fmt.Errorf("ReadData io.ReadFull failed,err:%w", err)
	}
	//fmt.Println("读到了", message)
	return string(message), nil
}

func WriteData(conn net.Conn, message string) error {
	//err := conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	//if err != nil {
	//	return err
	//}
	//设置读取消息的长度
	length := uint32(len(message))

	//以二进制形式将长度写入conn
	err := binary.Write(conn, binary.BigEndian, length)
	if err != nil {
		return fmt.Errorf("WriteData binary.Write failed,err:%w", err)
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		//if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		//	return errors.New("写数据超时")
		//}
		return fmt.Errorf("WriteData Write failed,err:%w", err)
	}
	return nil
}
