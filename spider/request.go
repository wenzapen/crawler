package spider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"math/rand"
	"time"
)

type Context struct {
	Body []byte
	Req  *Request
}

func (c *Context) GetRule(ruleName string) *Rule {
	return c.Req.Task.Rule.Trunk[ruleName]
}

func (c *Context) Output(data interface{}) *DataCell {
	dataCell := &DataCell{
		Task: c.Req.Task,
	}
	dataCell.Data = make(map[string]interface{})
	dataCell.Data["Task"] = c.Req.Task.Name
	dataCell.Data["Rule"] = c.Req.RuleName
	dataCell.Data["URL"] = c.Req.URL
	dataCell.Data["Time"] = time.Now().Format("2024-04-10 11-03-00")
	dataCell.Data["Data"] = data

	return dataCell
}

type Request struct {
	Task     *Task
	Method   string
	URL      string
	Depth    int64
	Priority int64
	RuleName string
	TmpData  *Temp
}

func (r *Request) Fetch() ([]byte, error) {
	if err := r.Task.Limit.Wait(context.Background()); err != nil {
		return nil, err
	}
	sleeptime := rand.Int63n(r.Task.WaitTime * 1000)
	time.Sleep(time.Duration(sleeptime) * time.Millisecond)
	return r.Task.Fetcher.Get(r)
}

type ParseResult struct {
	Requests []*Request
	Items    []interface{}
}

func (r *Request) Check() error {
	if r.Depth > r.Task.MaxDepth {
		return errors.New("Max depth limit is reached")
	}
	return nil
}

func (r *Request) Unique() string {
	block := md5.Sum([]byte(r.URL + r.Method))
	return hex.EncodeToString(block[:])
}
