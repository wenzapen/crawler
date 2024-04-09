package main

import (
	"fmt"
	"time"

	// "github.com/chromedp/chromedp"
	// "regexp"
	"github.com/wenzapen/crawler/collect"
	"github.com/wenzapen/crawler/engine"
	"github.com/wenzapen/crawler/log"
	"github.com/wenzapen/crawler/parse/doubangroup"
	"github.com/wenzapen/crawler/proxy"

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

	cookie := `bid=Hs5rbgnChtY; __utmz=30149280.1711842995.2.1.utmcsr=baidu|utmccn=(organic)|utmcmd=organic; viewed="1007305_35922722"; __gads=ID=6d018a364d5b9930:T=1711842995:RT=1712410663:S=ALNI_MbAdhio9yK6lCMueUG51CnH9UYbLw; __gpi=UID=00000d797b010d8e:T=1711842995:RT=1712410663:S=ALNI_MZ36mQJhokK4PXVzwdW0B6DLbcI_g; __eoi=ID=bc3d52e1797a7ba5:T=1711842995:RT=1712410663:S=AA-AfjbvuagX0mQXAWojlMkTq447; _pk_id.100001.8cb4=e7fc68e51292ae3c.1712497367.; __yadk_uid=TVpKYhmdIKh4uSZ4LweLR7nHQG5NFhca; douban-fav-remind=1; _pk_ses.100001.8cb4=1; ap_v=0,6.0; __utma=30149280.980901097.1656747039.1712499217.1712580541.8; __utmc=30149280; __utmt=1; __utmb=30149280.6.5.1712580541`

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 300 * time.Millisecond,
		Logger:  logger,
		Proxy:   p,
	}

	var seeds = make([]*collect.Task, 0, 1000)
	for i := 0; i <= 100; i += 25 {
		str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d,&type=new", i)
		seeds = append(seeds, &collect.Task{
			Url:      str,
			Cookie:   cookie,
			WaitTime: 3 * time.Second,
			MaxDepth: 5,
			Fetcher:  f,
			RootReq: &collect.Request{
				ParseFunction: doubangroup.ParseURL,
				Method:        "GET",
			},
		})
	}

	s := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithSeeds(seeds),
		engine.WithWorkCount(5),
		engine.WithScheduler(engine.NewSchedule()),
	)
	s.Run()

	// for len(seeds) > 0 {
	// 	items := seeds
	// 	seeds = nil
	// 	for _, item := range items {
	// 		body, err := f.Get(item)
	// 		if err != nil {
	// 			logger.Error("read content failed", zap.Error(err))
	// 			continue
	// 		}
	// 		res := item.ParseFunction(body, item)
	// 		for _, item := range res.Items {
	// 			logger.Info("result", zap.String("ger url:", item.(string)))
	// 		}
	// 		seeds = append(seeds, res.Requests...)

	// 	}
	// }

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
