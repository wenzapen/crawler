package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func main() {
	url := "https://www.thepaper.cn"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("fetch url %v failed", url)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("error status code %v", resp.StatusCode)
		// return
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("read content failed: %v", err)
		return
	}

	numLinks := strings.Count(string(body), "<a")
	fmt.Printf("home page has %d links\n", numLinks)
}
