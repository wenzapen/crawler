package proxy

import (
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
)

type ProxyFunc func(*http.Request) (*url.URL, error)

type roundRobinSwitcher struct {
	proxyURLs []*url.URL
	index     uint32
}

func (r *roundRobinSwitcher) GetProxy(pr *http.Request) (*url.URL, error) {
	index := atomic.AddUint32(&r.index, 1) - 1
	u := r.proxyURLs[index%uint32(len(r.proxyURLs))]
	return u, nil

}

func RoundRobinSwitcher(ProxyURLs ...string) (ProxyFunc, error) {
	if len(ProxyURLs) < 1 {
		return nil, errors.New("proxy url list is empty")
	}
	var urls []*url.URL
	for _, u := range ProxyURLs {
		parsedU, err := url.Parse(u)
		if err != nil {
			continue
		}
		urls = append(urls, parsedU)
	}
	return (&roundRobinSwitcher{urls, 0}).GetProxy, nil
}
