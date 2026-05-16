//go:build linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
)

type Request struct {
	ID   int            `json:"id"`
	Cmd  string         `json:"cmd"`
	Args []string       `json:"args"`
	Opts map[string]any `json:"opts,omitempty"`
}

type Response struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Stream string      `json:"stream,omitempty"`
	EOF    bool        `json:"eof,omitempty"`
	Error  *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func handleConn(conn net.Conn, ctx context.Context) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := io.Writer(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("thrived: read error: %v", err)
			}
			return
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(writer, req.ID, 1, "invalid JSON: "+err.Error())
			return
		}

		resp := dispatch(ctx, &req)
		data, err := json.Marshal(resp)
		if err != nil {
			sendError(writer, req.ID, 1, "marshal error: "+err.Error())
			return
		}

		if _, err := writer.Write(append(data, '\n')); err != nil {
			log.Printf("thrived: write error: %v", err)
			return
		}
	}
}

func sendError(w io.Writer, id int, code int, msg string) {
	resp := Response{ID: id, Error: &ErrorInfo{Code: code, Message: msg}}
	data, _ := json.Marshal(resp)
	w.Write(append(data, '\n'))
}