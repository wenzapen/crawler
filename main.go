package main

import (
	"fmt"
	"time"

	// "github.com/chromedp/chromedp"
	// "regexp"
	"github.com/wenzapen/crawler/collect"
	"github.com/wenzapen/crawler/log"
	"github.com/wenzapen/crawler/parse/doubangroup"
	"github.com/wenzapen/crawler/proxy"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// var headerRe = regexp.MustCompile(`<li class="\w*?" style="height.*?"><a target="_blank" href="/newsDetail_forward_\d{8}?" class="index_inherit__A1ImK">(.*?)</a></li>`)

func main() {

	plugin := log.NewStdoutPlugin(zapcore.InfoLevel)
	logger := log.NewLogger(plugin)
	logger.Info("logger inited")

	proxyURLs := []string{}
	p, err := proxy.RoundRobinSwitcher(proxyURLs...)
	if err != nil {
		logger.Error("RoundRobinSwitcher failed")
	}

	cookie := ""

	var worklist []*collect.Request
	for i := 25; i <= 100; i += 25 {
		str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d,&type=new", i)
		worklist = append(worklist, &collect.Request{
			Url:           str,
			Cookie:        cookie,
			ParseFunction: doubangroup.ParseURL,
		})
	}

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 300 * time.Millisecond,
		Proxy:   p,
	}

	for len(worklist) > 0 {
		items := worklist
		worklist = nil
		for _, item := range items {
			body, err := f.Get(item)
			if err != nil {
				logger.Error("read content failed", zap.Error(err))
				continue
			}
			res := item.ParseFunction(body, item)
			for _, item := range res.Items {
				logger.Info("result", zap.String("ger url:", item.(string)))
			}
			worklist = append(worklist, res.Requests...)

		}
	}

	// url := "https://www.thepaper.cn"
	// url := "https://book.douban.com/subject/1007305/"
	// var bf collect.Fetcher = collect.BaseFetch{}
	// var bf collect.BrowserFetch = collect.BrowserFetch{}
	// resp, err := bf.Get(url)
	// if err != nil {
	// 	fmt.Printf("read content failed %v", err)
	// 	return
	// }
	// fmt.Println(string(resp))
	// matches := headerRe.FindAllSubmatch(resp, -1)
	// for _, m := range matches {
	// 	fmt.Println("fetch card news:", string(m[1]))
	// }

	// ctx, cancel := chromedp.NewContext(context.Background())
	// defer cancel()
	// ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	// defer cancel()

	// var example string
	// err := chromedp.Run(ctx,
	// 	chromedp.Navigate(`https://pkg.go.dev/time`),
	// 	chromedp.WaitVisible(`body>footer`),
	// 	chromedp.Click(`#example-After`, chromedp.NodeVisible),
	// 	chromedp.Value(`#example-After textarea`, &example))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Printf("Go's time.After example:\\n%s", example)
}
