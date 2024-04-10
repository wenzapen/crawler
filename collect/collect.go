package collect

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/wenzapen/crawler/proxy"
	"go.uber.org/zap"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type BaseFetch struct {
}

func (b BaseFetch) Get(req *Request) ([]byte, error) {
	resp, err := http.Get(req.Url)
	if err != nil {
		fmt.Printf("fetch url %v failed", req.Url)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("error status code %v", resp.StatusCode)
		return nil, err
	}

	bodyReader := bufio.NewReader(resp.Body)

	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())

	return ioutil.ReadAll(utf8Reader)
}

type BrowserFetch struct {
	Timeout time.Duration
	Proxy   proxy.ProxyFunc
	Logger  *zap.Logger
}

func (b BrowserFetch) Get(request *Request) ([]byte, error) {

	if request == nil {
		b.Logger.Sugar().Error("invalid request")
		return nil, errors.New("invalid request")
	}
	cli := &http.Client{
		Timeout: b.Timeout,
	}
	if b.Proxy != nil {
		transport := http.DefaultTransport.(*http.Transport)
		transport.Proxy = b.Proxy
		cli.Transport = transport
	}
	// fmt.Println("url:", request.Url)
	b.Logger.Info(request.Url)
	req, err := http.NewRequest("GET", request.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("get url failed:%v", err)
	}
	if len(request.Cookie) > 0 {
		req.Header.Set("Cookie", request.Cookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
	resp, err := cli.Do(req)

	time.Sleep(request.WaitTime)

	if err != nil {
		b.Logger.Error("fetch failed", zap.Error(err))
		return nil, err
	}
	bodyReader := bufio.NewReader(resp.Body)
	// bodyReader.ReadBytes()

	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())

	return ioutil.ReadAll(utf8Reader)
}

func DeterminEncoding(r *bufio.Reader) encoding.Encoding {
	bytes, err := r.Peek(1024)
	if err != nil {
		fmt.Printf("fetch error %v", err)
		return unicode.UTF8
	}
	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}
