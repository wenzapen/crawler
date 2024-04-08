package doubangroup

import (
	"regexp"

	"github.com/wenzapen/crawler/collect"
)

const urlListRe = `<a href="(https://www.douban.com/group/topic/\d+?/)" title="[\s\S]*?" class="">[\s\S]*?</a>`

func ParseURL(content []byte, req *collect.Request) collect.ParseResult {
	re := regexp.MustCompile(urlListRe)
	matches := re.FindAllSubmatch(content, -1)
	result := collect.ParseResult{}

	for _, m := range matches {
		u := string(m[1])
		result.Requests = append(result.Requests, &collect.Request{
			Url:    u,
			Cookie: req.Cookie,
			ParseFunction: func(c []byte, r *collect.Request) collect.ParseResult {
				return GetContent(c, r)
			},
		})
	}
	return result
}

const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div`

func GetContent(content []byte, request *collect.Request) collect.ParseResult {
	re := regexp.MustCompile(ContentRe)
	ok := re.Match(content)
	if !ok {
		return collect.ParseResult{
			Items: []interface{}{},
		}
	}
	result := collect.ParseResult{
		Items: []interface{}{request.Url},
	}
	return result

}
