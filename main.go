package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	os.Setenv("HTTPS_PROXY", "http://10.144.1.10:8080")
	os.Setenv("HTTP_PROXY", "http://10.144.1.10:8080")
	url := "https://www.thepaper.cn/"
	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("fetch url error:%v", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error status code:%v", resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read content failed:%v", err)
		return
	}

	fmt.Println("body:", string(body))
}
