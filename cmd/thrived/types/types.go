//go:build linux

package types

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