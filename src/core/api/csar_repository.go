/*
@Time : 2021/6/4
@Author : jzd
@Project: harbor
*/
package api

import (
	"errors"
	"fmt"
	"github.com/goharbor/harbor/src/core/label"
	"github.com/goharbor/harbor/src/csar"
	hlog "github.com/goharbor/harbor/src/lib/log"
	"github.com/goharbor/harbor/src/server/middleware/orm"
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

//reserve proxy
func (csar *CsarRepositoryAPI) Proxy() {
	// Check access
	if !csar.SecurityCtx.IsAuthenticated() {
		csar.SendUnAuthorizedError(errors.New("Unauthorized"))
		return
	}
	if !csar.SecurityCtx.IsSysAdmin() {
		csar.SendForbiddenError(errors.New(csar.SecurityCtx.GetUsername()))
		return
	}
	// Directly proxy to the backend
	csarController.ProxyTraffic(csar.Ctx.ResponseWriter, csar.Ctx.Request)
}
