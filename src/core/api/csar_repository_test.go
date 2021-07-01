/*
@Time : 2021/6/28
@Author : jzd
@Project: harbor
*/
package api

import (
	"github.com/goharbor/harbor/src/replication/model"
	"net/http"
	"testing"
)

func TestProxy(t *testing.T) {
	csarVersions := make([]*model.Repository, 0)
	err := handleAndParse(&testingRequest{
		url:        "/api/csarrepo/test/csars",
		method:     http.MethodGet,
		credential: projAdmin,
	}, &csarVersions)

	if err != nil {
		t.Fatal(err)
	}

	if len(csarVersions) != 2 {
		t.Fatalf("expect 2 chart versions but got %d", len(csarVersions))
	}
}
