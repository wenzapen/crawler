package main

import (
	"fmt"

	// "regexp"

	"github.com/wenzapen/crawler/collect"
)

// var headerRe = regexp.MustCompile(`<li class="\w*?" style="height.*?"><a target="_blank" href="/newsDetail_forward_\d{8}?" class="index_inherit__A1ImK">(.*?)</a></li>`)

func main() {
	// url := "https://www.thepaper.cn"
	url := "https://book.douban.com/subject/1007305/"
	// var bf collect.Fetcher = collect.BaseFetch{}
	var bf collect.BrowserFetch = collect.BrowserFetch{}
	resp, err := bf.Get(url)
	if err != nil {
		fmt.Printf("read content failed %v", err)
		return
	}
	fmt.Println(string(resp))
	// matches := headerRe.FindAllSubmatch(resp, -1)
	// for _, m := range matches {
	// 	fmt.Println("fetch card news:", string(m[1]))
	// }
}
