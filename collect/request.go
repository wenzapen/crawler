package collect

type Request struct {
	Url           string
	Cookie        string
	ParseFunction func([]byte, *Request) ParseResult
}

type ParseResult struct {
	Requests []*Request
	Items    []interface{}
}
