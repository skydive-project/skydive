package gremlin

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

type Response struct {
	RequestId string          `json:"requestId"`
	Status    *ResponseStatus `json:"status"`
	Result    *ResponseResult `json:"result"`
}

type ResponseStatus struct {
	Code       int                    `json:"code"`
	Attributes map[string]interface{} `json:"attributes"`
	Message    string                 `json:"message"`
}

type ResponseResult struct {
	Data json.RawMessage        `json:"data"`
	Meta map[string]interface{} `json:"meta"`
}

func ReadResponse(ws *websocket.Conn) (data []byte, err error) {
	// Data buffer
	var message []byte
	var dataItems []json.RawMessage
	inBatchMode := false
	// Receive data
	for {
		if _, message, err = ws.ReadMessage(); err != nil {
			return
		}
		var res *Response
		if err = json.Unmarshal(message, &res); err != nil {
			return
		}
		var items []json.RawMessage
		switch res.Status.Code {
		case StatusNoContent:
			return

		case StatusPartialContent:
			inBatchMode = true
			if err = json.Unmarshal(res.Result.Data, &items); err != nil {
				return
			}
			dataItems = append(dataItems, items...)

		case StatusSuccess:
			if inBatchMode {
				if err = json.Unmarshal(res.Result.Data, &items); err != nil {
					return
				}
				dataItems = append(dataItems, items...)
				data, err = json.Marshal(dataItems)
			} else {
				data = res.Result.Data
			}
			return

		default:
			if msg, exists := ErrorMsg[res.Status.Code]; exists {
				err = errors.New(msg)
			} else {
				err = errors.New("An unknown error occured")
			}
			return
		}
	}
	return
}

func (req *Request) Exec() (data []byte, err error) {
	// Prepare the Data
	message, err := json.Marshal(req)
	if err != nil {
		return
	}
	// Prepare the request message
	var requestMessage []byte
	mimeType := []byte("application/json")
	mimeTypeLen := byte(len(mimeType))
	requestMessage = append(requestMessage, mimeTypeLen)
	requestMessage = append(requestMessage, mimeType...)
	requestMessage = append(requestMessage, message...)
	// Open a TCP connection
	conn, server, err := CreateConnection()
	if err != nil {
		return
	}
	// Open a new socket connection
	ws, _, err := websocket.NewClient(conn, server, http.Header{}, 0, len(requestMessage))
	if err != nil {
		return
	}
	defer ws.Close()
	if err = ws.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if err = ws.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if err = ws.WriteMessage(websocket.BinaryMessage, requestMessage); err != nil {
		return
	}

	return ReadResponse(ws)
}
