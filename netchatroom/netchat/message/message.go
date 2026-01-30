package message

import (
	"encoding/json"
	"fmt"
	"net"
	"netchatroom/netchat/common"
	"netchatroom/netchat/utils"
)

const (
	Register = iota
	Login
	Join
	Quit
	CheckUser
	CheckRankList
	PrivateMsg
	PublicMsg
	HeartMsg
	PublicHistory
	PrivateHistory
)

func MsgToJson(message *common.Message) (string, error) {
	msg, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("Marshal failed,err:%w", err)
	}
	return string(msg), nil
}

func JsonToMsg(msg string) (*common.Message, error) {
	message := &common.Message{}
	err := json.Unmarshal([]byte(msg), message)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal failed ,err:%w", err)
	}
	return message, nil
}

func SendMsg(conn net.Conn, message *common.Message) error {
	msg, err := MsgToJson(message)
	if err != nil {
		return fmt.Errorf("MsgToJson failed,err:%w", err)
	}
	err = utils.WriteData(conn, msg)
	if err != nil {
		return fmt.Errorf("WriteData failed,err:%w", err)
	}
	return nil
}

func ReciveMsg(conn net.Conn) (*common.Message, error) {
	msg, err := utils.ReadData(conn)
	if err != nil {
		return nil, fmt.Errorf("ReadData failed ,err:%w", err)
	}
	message, err := JsonToMsg(msg)
	if err != nil {
		return nil, fmt.Errorf("JsonToMsg failed ,err:%w", err)
	}
	return message, nil

}
