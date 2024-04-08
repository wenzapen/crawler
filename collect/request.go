package collect

import "time"

type Request struct {
	Url           string
	Cookie        string
	WaitTime      time.Duration
	ParseFunction func([]byte, *Request) ParseResult
}

type ParseResult struct {
	Requests []*Request
	Items    []interface{}
}
