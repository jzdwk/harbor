/*
@Time : 2021/6/4
@Author : jzd
@Project: harbor
*/
package api

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/goharbor/harbor/src/common/rbac"
	"github.com/goharbor/harbor/src/core/label"
	"github.com/goharbor/harbor/src/csar"
	hlog "github.com/goharbor/harbor/src/lib/log"
	"github.com/goharbor/harbor/src/server/middleware/orm"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// chartController is a singleton instance
var csarController *csar.Controller

// csarController returns the chart controller
func GetCsarController() *csar.Controller {
	return csarController
}

// CsarRepositoryAPI provides related API handlers for the csar repository APIs
type CsarRepositoryAPI struct {
	// The base controller to provide common utilities
	BaseController

	// For label management
	labelManager *label.BaseManager

	// Keep the namespace if existing
	namespace string
}

// Prepare something for the following actions
func (csar *CsarRepositoryAPI) Prepare() {
	// Call super prepare method
	csar.BaseController.Prepare()
	// Try to extract namespace for parameter of path
	// It may not exist
	csar.namespace = strings.TrimSpace(csar.GetStringFromPath(namespaceParam))
	if !csar.requireNamespace() {
		hlog.Error("csar controller namespace does not exist, namespace: %v", csar.namespace)
	}
	hlog.Info("setting csar controller namespace  %v", csar.namespace)

	// Init label manager
	csar.labelManager = &label.BaseManager{}
}

// Check if there exists a valid namespace
// Return true if it does
// Return false if it does not
func (csar *CsarRepositoryAPI) requireNamespace() bool {
	hlog.Info("check csar controller namespace  %v", csar.namespace)
	namespace := csar.namespace
	// Actually, never should be like this
	if len(namespace) == 0 {
		csar.SendBadRequestError(errors.New(":repo should be in the request URL"))
		return false
	}

	existing, err := csar.ProjectCtl.Exists(csar.Context(), namespace)
	if err != nil {
		// Check failed with error
		csar.SendInternalServerError(fmt.Errorf("failed to check existence of namespace %s with error: %s", namespace, err.Error()))
		return false
	}

	// Not existing
	if !existing {
		csar.handleProjectNotFound(namespace)
		return false
	}

	return true
}

const defaultCsarEndPoint = "http://192.168.182.133:8070"

func GetCsarEndPoint() string {
	endpoint := os.Getenv("CSAR_ENDPOINT")
	if endpoint == "" {
		return defaultCsarEndPoint
	}
	return endpoint
}

// Initialize the chart service controller
func initializeCsarController() (*csar.Controller, error) {
	csarEndPoint := GetCsarEndPoint()
	csarEndPoint = strings.TrimSuffix(csarEndPoint, "/")
	url, err := url.Parse(csarEndPoint)
	if err != nil {
		return nil, errors.New("Endpoint URL of csar storage server is malformed")
	}
	controller, err := csar.NewController(url, orm.Middleware())
	if err != nil {
		return nil, errors.New("Failed to initialize csar API controller")
	}
	hlog.Debugf("Csar storage server is set to %s", url.String())
	hlog.Info("API controller for csar repository server is successfully initialized")
	return controller, nil
}

func (csar *CsarRepositoryAPI) requireAccess(action rbac.Action, subresource ...rbac.Resource) bool {
	if len(subresource) == 0 {
		subresource = append(subresource, rbac.ResourceCsar)
	}

	return csar.RequireProjectAccess(csar.namespace, action, subresource...)
}

func (csar *CsarRepositoryAPI) Upload() {
	hlog.Debugf("Header of request of uploading csar: %#v, content-len=%d", csar.Ctx.Request.Header, csar.Ctx.Request.ContentLength)

	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}

	// Check access
	if !csar.requireAccess(rbac.ActionCreate, rbac.ResourceCsar) {
		return
	}

	// Rewrite file content if the content type is "multipart/form-data"
	if isMultipartFormData(csar.Ctx.Request) {
		formFiles := make([]formFile, 0)
		formFiles = append(formFiles,
			formFile{
				formField: "csar",
				mustHave:  true,
			})
		if err := csar.rewriteFileContent(formFiles, csar.Ctx.Request); err != nil {
			csar.SendInternalServerError(err)
			return
		}
		/*if err := csar.addEventContext(formFiles, cra.Ctx.Request); err != nil {
			hlog.Errorf("Failed to add chart upload context, %v", err)
		}*/
	}

	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}

func (csar *CsarRepositoryAPI) Get() {
	hlog.Infof("get request from get detail api")
	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}
	// Check access
	if !csar.requireAccess(rbac.ActionRead, rbac.ResourceCsar) {
		return
	}
	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}

func (csar *CsarRepositoryAPI) List() {
	hlog.Infof("get request from get list api")
	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}
	// Check access
	if !csar.requireAccess(rbac.ActionList, rbac.ResourceCsar) {
		return
	}
	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}

func (csar *CsarRepositoryAPI) Delete() {
	hlog.Infof("get request from delete api")
	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}
	// Check access
	if !csar.requireAccess(rbac.ActionDelete, rbac.ResourceCsar) {
		return
	}
	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}

func (csar *CsarRepositoryAPI) Download() {
	hlog.Infof("get request from download api")
	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}
	// Check access
	if !csar.requireAccess(rbac.ActionRead, rbac.ResourceCsar) {
		return
	}
	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}

func (csar *CsarRepositoryAPI) rewriteFileContent(files []formFile, request *http.Request) error {
	if len(files) == 0 {
		return nil // no files, early return
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			// Just log it
			hlog.Errorf("Failed to defer close multipart writer with error: %s", err.Error())
		}
	}()

	// Process files by key one by one
	for _, f := range files {
		mFile, mHeader, err := csar.GetFile(f.formField)

		// Handle error case by case
		if err != nil {
			formatedErr := fmt.Errorf("get file content with multipart header from key '%s' failed with error: %s", f.formField, err.Error())
			if f.mustHave || err != http.ErrMissingFile {
				return formatedErr
			}

			// Error can be ignored, just log it
			hlog.Warning(formatedErr.Error())
			continue
		}

		fw, err := w.CreateFormFile(f.formField, mHeader.Filename)
		if err != nil {
			return fmt.Errorf("create form file with multipart header failed with error: %s", err.Error())
		}

		_, err = io.Copy(fw, mFile)
		if err != nil {
			return fmt.Errorf("copy file stream in multipart form data failed with error: %s", err.Error())
		}

	}
	request.Header.Set(headerContentType, w.FormDataContentType())
	request.ContentLength = -1
	request.Body = ioutil.NopCloser(&body)
	return nil
}
