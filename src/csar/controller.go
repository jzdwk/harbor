/*
@Time : 2021/6/4
@Author : jzd
@Project: harbor
*/
package csar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/goharbor/harbor/src/common"
	commonhttp "github.com/goharbor/harbor/src/common/http"
	"github.com/goharbor/harbor/src/controller/event/metadata"
	hlog "github.com/goharbor/harbor/src/lib/log"
	n_event "github.com/goharbor/harbor/src/pkg/notifier/event"
	"github.com/goharbor/harbor/src/replication"
	rep_event "github.com/goharbor/harbor/src/replication/event"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	userName    = "csar_controller"
	passwordKey = "CORE_SECRET"
)

// Credential keeps the username and password for the basic auth
type Credential struct {
	Username string
	Password string
}

// Controller is used to handle flows of related requests based on the corresponding handlers
// A reverse proxy will be created and managed to proxy the related traffics between API and
// backend csar server
type Controller struct {
	proxy http.Handler
}

// NewController is constructor of the csar.Controller
func NewController(backendServer *url.URL, middlewares ...func(http.Handler) http.Handler) (*Controller, error) {
	// Try to create credential
	var engine http.Handler
	// Try to create credential
	cred := &Credential{
		Username: userName,
		Password: os.Getenv(passwordKey),
	}
	engine = &httputil.ReverseProxy{
		ErrorLog: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile),
		Director: func(req *http.Request) {
			director(backendServer, cred, req)
		},
		ModifyResponse: modifyResponse,
		Transport:      commonhttp.GetHTTPTransport(commonhttp.SecureTransport),
	}

	if len(middlewares) > 0 {
		hlog.Info("New csar server traffic proxy with middlewares")
		for i := len(middlewares) - 1; i >= 0; i-- {
			engine = middlewares[i](engine)
		}
	}

	return &Controller{
		proxy: engine,
	}, nil
}

// ProxyTraffic implements the interface method.
func (c *Controller) ProxyTraffic(w http.ResponseWriter, req *http.Request) {
	hlog.Info("csar server controller: original request uri %v", req.RequestURI)
	// do proxy
	c.proxy.ServeHTTP(w, req)

}

// Overwrite the http requests
func director(target *url.URL, cred *Credential, req *http.Request) {
	// Closure
	targetQuery := target.RawQuery

	// Overwrite the request URL to the target path
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	//csar server api equals to harbor csar api, so remove rewrite
	//rewriteURLPath(req)
	req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "HARBOR")
	}
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	// Add authentication header if it is existing
	if cred != nil {
		req.SetBasicAuth(cred.Username, cred.Password)
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// Modify the http response
func modifyResponse(res *http.Response) error {
	// Upload csar success, then to the notification to replication handler
	if res.StatusCode == http.StatusCreated {
		// 201 and has csar_upload_event context
		// means this response is for uploading chart success.
		hlog.Infof("create csar success, starting web hook  ")
		csarUploadEvent := res.Request.Context().Value(common.CsarUploadCtxKey)
		e, ok := csarUploadEvent.(*rep_event.Event)
		if !ok {
			hlog.Error("failed to convert csar upload context into replication event.")
		} else {
			// Todo: it used as the replacement of webhook, will be removed when webhook to be introduced.
			go func() {
				if err := replication.EventHandler.Handle(e); err != nil {
					hlog.Errorf("failed to handle event: %v", err)
				}
			}()

			// Trigger harbor webhook
			if e != nil && e.Resource != nil && e.Resource.Metadata != nil &&
				len(e.Resource.ExtendedInfo) > 0 {
				event := &n_event.Event{}
				metaData := &metadata.CsarUploadMetaData{
					CsarMetaData: metadata.CsarMetaData{
						ProjectName: e.Resource.ExtendedInfo["projectName"].(string),
						CsarName:    e.Resource.ExtendedInfo["csarName"].(string),
						//Versions:    e.Resource.Metadata.Artifacts[0].Tags,
						OccurAt:  time.Now(),
						Operator: e.Resource.ExtendedInfo["operator"].(string),
					},
				}
				if err := event.Build(metaData); err == nil {
					if err := event.Publish(); err != nil {
						hlog.Errorf("failed to publish chart upload event: %v", err)
					}
				} else {
					hlog.Errorf("failed to build chart upload event metadata: %v", err)
				}
			}
		}
	}

	// Process downloading chart success webhook event
	if res.StatusCode == http.StatusOK {
		hlog.Infof("download csar success, starting web hook  ")
		csarDownloadEvent := res.Request.Context().Value(common.CsarDownloadCtxKey)
		eventMetaData, ok := csarDownloadEvent.(*metadata.CsarDownloadMetaData)
		if ok && eventMetaData != nil {
			// Trigger harbor webhook
			event := &n_event.Event{}
			if err := event.Build(eventMetaData); err == nil {
				if err := event.Publish(); err != nil {
					hlog.Errorf("failed to publish csar download event: %v", err)
				}
			} else {
				hlog.Errorf("failed to build csar download event metadata: %v", err)
			}
		}
	}
	//process delete csar succuss webhook event
	if res.StatusCode == http.StatusNoContent {
		hlog.Infof("delete csar success, starting web hook  ")
		csarDeleteEvent := res.Request.Context().Value(common.CsarDeleteCtxKey)
		eventMetaData, ok := csarDeleteEvent.(*metadata.CsarDeleteMetaData)
		if ok && eventMetaData != nil {
			// Trigger harbor webhook
			event := &n_event.Event{}
			if err := event.Build(eventMetaData); err == nil {
				if err := event.Publish(); err != nil {
					hlog.Errorf("failed to publish csar delete event: %v", err)
				}
			} else {
				hlog.Errorf("failed to build csar delete event metadata: %v", err)
			}
		}
	}

	// Accept cases
	// Success or redirect
	if res.StatusCode >= http.StatusOK && res.StatusCode <= http.StatusTemporaryRedirect {
		return nil
	}

	// Detect the 401 code, if it is,overwrite it to 500.
	// We also re-write the error content to structural error object
	errorObj := make(map[string]string)
	if res.StatusCode == http.StatusUnauthorized {
		errorObj["error"] = "operation request from unauthorized source is rejected"
		res.StatusCode = http.StatusInternalServerError
	} else {
		// Extract the error and wrap it into the error object
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			errorObj["error"] = fmt.Sprintf("%s: %s", res.Status, err.Error())
		} else {
			if err := json.Unmarshal(data, &errorObj); err != nil {
				errorObj["error"] = string(data)
			}
		}
	}

	content, err := json.Marshal(errorObj)
	if err != nil {
		return err
	}

	size := len(content)
	body := ioutil.NopCloser(bytes.NewReader(content))
	res.Body = body
	res.ContentLength = int64(size)
	res.Header.Set("Content-Length", strconv.Itoa(size))

	return nil
}
