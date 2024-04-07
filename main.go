package main

import (
	"fmt"

	// "github.com/chromedp/chromedp"
	// "regexp"
	"github.com/wenzapen/crawler/collect"
)

// var headerRe = regexp.MustCompile(`<li class="\w*?" style="height.*?"><a target="_blank" href="/newsDetail_forward_\d{8}?" class="index_inherit__A1ImK">(.*?)</a></li>`)

func main() {

	var worklist []*collect.Request
	for i := 25; i <= 100; i += 25 {
		str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d,&type=new", i)
		worklist = append(worklist, &collect.Request{
			Url:           str,
			ParseFunction: ParseCityList,
		})
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
