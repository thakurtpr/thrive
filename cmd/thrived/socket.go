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
	ID     int        `json:"id"`
	Result any        `json:"result,omitempty"`
	Stream string     `json:"stream,omitempty"`
	EOF    bool       `json:"eof,omitempty"`
	Error  *ErrorInfo `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeResponse(w io.Writer, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("thrived: marshal error: %v", err)
		return
	}
	w.Write(append(data, '\n'))
}

func sendError(w io.Writer, id int, code int, msg string) {
	writeResponse(w, &Response{ID: id, Error: &ErrorInfo{Code: code, Message: msg}})
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
			sendError(writer, 0, 1, "invalid JSON: "+err.Error())
			return
		}

		// dispatch writes directly to writer (supports streaming handlers)
		dispatch(ctx, &req, writer)
	}
}
