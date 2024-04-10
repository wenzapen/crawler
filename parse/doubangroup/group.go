package doubangroup

import (
	"fmt"
	"regexp"

	"github.com/wenzapen/crawler/spider"
)

const urlListRe = `<a href="(https://www.douban.com/group/topic/\d+?/)" title="[\s\S]*?" class="">[\s\S]*?</a>`
const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div`

var DoubanGroupTask = &spider.Task{
	Rule: spider.RuleTree{
		Root: func() ([]*spider.Request, error) {
			var roots []*spider.Request
			for i := 0; i <= 100; i += 25 {
				str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d,&type=new", i)
				roots = append(roots, &spider.Request{
					URL:      str,
					Method:   "GET",
					Priority: 1,
					RuleName: "PareseURL",
				})
			}
			return roots, nil
		},
		Trunk: map[string]*spider.Rule{
			"ParseURL":       {ParseFunc: ParseURL},
			"ParseSunnyRoom": {ParseFunc: GetSunnyRoom},
		},
	},
}

func ParseURL(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(urlListRe)
	matches := re.FindAllSubmatch(ctx.Body, -1)
	result := spider.ParseResult{}

	for _, m := range matches {
		u := string(m[1])
		fmt.Println("url: ", u)
		result.Requests = append(result.Requests, &spider.Request{
			Method:   "GET",
			URL:      u,
			Task:     ctx.Req.Task,
			Depth:    ctx.Req.Depth + 1,
			RuleName: "ParseSunnyRoom",
		})
	}
	return result, nil
}

func GetSunnyRoom(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(ContentRe)
	ok := re.Match(ctx.Body)
	if !ok {
		return spider.ParseResult{
			Items: []interface{}{},
		}, nil
	}
	result := spider.ParseResult{
		Items: []interface{}{ctx.Req.URL},
	}
	return result, nil

}
