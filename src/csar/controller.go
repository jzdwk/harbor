/*
@Time : 2021/6/4
@Author : jzd
@Project: harbor
*/
package csar

import (
	hlog "github.com/goharbor/harbor/src/lib/log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Credential keeps the username and password for the basic auth
type Credential struct {
	Username string
	Password string
}

// Controller is used to handle flows of related requests based on the corresponding handlers
// A reverse proxy will be created and managed to proxy the related traffics between API and
// backend chart server
type Controller struct {
	target *url.URL
}

// NewController is constructor of the chartserver.Controller
func NewController(backendServer *url.URL, middlewares ...func(http.Handler) http.Handler) (*Controller, error) {
	return &Controller{
		backendServer,
	}, nil
}

// ProxyTraffic implements the interface method.
func (c *Controller) ProxyTraffic(w http.ResponseWriter, req *http.Request) {
	hlog.Info("csar server controller: original request uri %v", req.RequestURI)
	// create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(c.target)
	// update  headers
	req.URL.Host = c.target.Host
	req.URL.Scheme = c.target.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = c.target.Host
	// do proxy
	proxy.ServeHTTP(w, req)
}
