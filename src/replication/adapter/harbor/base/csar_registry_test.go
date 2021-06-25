/*
@Time : 2021/6/25
@Author : jzd
@Project: harbor
*/
package base

import (
	"bytes"
	"github.com/goharbor/harbor/src/common/utils/test"
	"github.com/goharbor/harbor/src/replication/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestCsarExist(t *testing.T) {
	//create http server for test
	server := test.NewServer(&test.RequestHandlerMapping{
		Method:  http.MethodGet,
		Pattern: "/api/csarrepo/test/csars/kong",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			data := `{
				"metadata": {
					"urls":["http://127.0.0.1/charts"]
				}
			}`
			w.Write([]byte(data))
		},
	})
	defer server.Close()
	registry := &model.Registry{
		URL: server.URL,
	}
	adapter, err := New(registry)
	require.Nil(t, err)
	exist, err := adapter.CsarExist("test/kong")
	require.Nil(t, err)
	require.True(t, exist)
}

func TestFetchCsar(t *testing.T) {
	server := test.NewServer([]*test.RequestHandlerMapping{
		{
			Method:  http.MethodGet,
			Pattern: "/api/projects",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				data := `[{
					"name": "test",
					"metadata": {"public":true}
				}]`
				w.Write([]byte(data))
			},
		},
		{
			Method:  http.MethodGet,
			Pattern: "/api/csarrepo/test/csars/kong",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				data := `{
				"version":"2.0",
				"labels":[{
					"name":"test"
				}]
				}`
				w.Write([]byte(data))
			},
		},
		{
			Method:  http.MethodGet,
			Pattern: "/api/csarrepo/test/csars",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				data := `[{
				"name": "kong"
			}]`
				w.Write([]byte(data))
			},
		},
	}...)
	defer server.Close()
	registry := &model.Registry{
		URL: server.URL,
	}
	adapter, err := New(registry)
	require.Nil(t, err)

	// not nil filter
	filters := []*model.Filter{
		{
			Type:  model.FilterTypeName,
			Value: "test/*",
		},
	}
	resources, err := adapter.FetchCsar(filters)
	require.Nil(t, err)
	require.Equal(t, 1, len(resources))
}

func TestDownloadCsar(t *testing.T) {
	server := test.NewServer([]*test.RequestHandlerMapping{
		{
			Method:  http.MethodGet,
			Pattern: "/api/csarrepo/test/csars/kong",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				data := `{
				"version":"2.0",
				"labels":[{
					"name":"test"
				}]
				}`
				w.Write([]byte(data))
			},
		},
		{
			Method:  http.MethodGet,
			Pattern: "/api/csarrepo/test/charts/kong/download",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}...)
	defer server.Close()
	registry := &model.Registry{
		URL: server.URL,
	}
	adapter, err := New(registry)
	require.Nil(t, err)
	_, err = adapter.DownloadCsar("test/kong")
	require.Nil(t, err)
}

func TestUploadCsar(t *testing.T) {
	server := test.NewServer(&test.RequestHandlerMapping{
		Method:  http.MethodPost,
		Pattern: "/api/csarrepo/test/csars",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})
	defer server.Close()
	registry := &model.Registry{
		URL: server.URL,
	}
	adapter, err := New(registry)
	require.Nil(t, err)
	err = adapter.UploadCsar("test/kong", bytes.NewBuffer(nil))
	require.Nil(t, err)
}

func TestDeleteCsar(t *testing.T) {
	server := test.NewServer(&test.RequestHandlerMapping{
		Method:  http.MethodDelete,
		Pattern: "/api/csarrepo/test/csars/kong",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})
	defer server.Close()
	registry := &model.Registry{
		URL: server.URL,
	}
	adapter, err := New(registry)
	require.Nil(t, err)
	err = adapter.DeleteCsar("test/kong")
	require.Nil(t, err)
}
