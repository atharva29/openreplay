package service

import (
	"context"
	"fmt"
	"openreplay/backend/pkg/messages"
	"strconv"
)

func (v *ImageStorage) MessageIterator(data []byte, sessID uint64) {
	checkSessionEnd := func(data []byte) (messages.Message, error) {
		reader := messages.NewBytesReader(data)
		msgType, err := reader.ReadUint()
		if err != nil {
			return nil, err
		}
		if msgType != messages.MsgMobileSessionEnd {
			return nil, fmt.Errorf("not a mobile session end message")
		}
		msg, err := messages.ReadMessage(msgType, reader)
		if err != nil {
			return nil, fmt.Errorf("read message err: %s", err)
		}
		return msg, nil
	}
	isCleanSessionEvent := func(data []byte) bool {
		reader := messages.NewBytesReader(data)
		msgType, err := reader.ReadUint()
		if err != nil {
			return false
		}
		if msgType != messages.MsgCleanSession {
			return false
		}
		_, err = messages.ReadMessage(msgType, reader)
		if err != nil {
			return false
		}
		return true
	}
	sessCtx := context.WithValue(context.Background(), "sessionID", fmt.Sprintf("%d", sessID))

	if _, err := checkSessionEnd(data); err == nil {
		if err := v.PackScreenshots(sessCtx, sessID, v.cfg.FSDir+"/screenshots/"+strconv.FormatUint(sessID, 10)+"/"); err != nil {
			v.log.Error(sessCtx, "can't pack screenshots: %s", err)
		}
	} else if isCleanSessionEvent(data) {
		if err := v.CleanSession(sessCtx, sessID); err != nil {
			v.log.Error(sessCtx, "can't clean session: %s", err)
		}
	} else {
		if err := v.Process(sessCtx, sessID, data); err != nil {
			v.log.Error(sessCtx, "can't process screenshots: %s", err)
		}
	}
}
