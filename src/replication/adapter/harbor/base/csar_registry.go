/*
@Time : 2021/6/8
@Author : jzd
@Project: harbor
*/
package base

import (
	"bytes"
	"fmt"
	common_http "github.com/goharbor/harbor/src/common/http"
	"github.com/goharbor/harbor/src/common/utils"
	"github.com/goharbor/harbor/src/replication/filter"
	"github.com/goharbor/harbor/src/replication/model"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
)

type csarDetail struct {
	Version string   `json:"version"`
	Labels  []*label `json:"labels"`
}

type csarMetadata struct {
	Metadata interface{} `json:"metadata"`
}

func (a *Adapter) FetchCsar(filters []*model.Filter) ([]*model.Resource, error) {
	projects, err := a.ListProjects(filters)
	if err != nil {
		return nil, err
	}
	resources := []*model.Resource{}
	for _, project := range projects {
		//get all csars in this project
		//e. httpbin  busybox
		url := fmt.Sprintf("%s/api/csarrepo/%s/csars", a.Client.GetURL(), project.Name)
		repositories := []*model.Repository{}
		if err := a.httpClient.Get(url, &repositories); err != nil {
			return nil, err
		}
		if len(repositories) == 0 {
			continue
		}
		for _, repository := range repositories {
			//e. project test, repo httpbin, Name=test/httpbin
			repository.Name = fmt.Sprintf("%s/%s", project.Name, repository.Name)
		}
		//filter={type:name value:httpbin/*}
		//repo=httpbin
		repositories, err = filter.DoFilterRepositories(repositories, filters)
		if err != nil {
			return nil, err
		}
		for _, repository := range repositories {
			name := strings.SplitN(repository.Name, "/", 2)[1]
			//GET /api/csarrepo/test/csars/httpbin
			url := fmt.Sprintf("%s/api/csarrepo/%s/csars/%s", a.Client.GetURL(), project.Name, name)
			//todo define more info about csar detail
			//httpbin v1.0 v2.0
			detail := &csarDetail{}
			if err := a.httpClient.Get(url, &detail); err != nil {
				return nil, err
			}
			if detail == nil {
				continue
			}
			var artifacts []*model.Artifact
			var labels []string
			for _, label := range detail.Labels {
				labels = append(labels, label.Name)
			}
			//artifact = {tags:v1.0, labels:xxx}
			artifacts = append(artifacts, &model.Artifact{
				Tags:   []string{detail.Version},
				Labels: labels,
			})
			//filter, filter lable,tag,resource, do nothing
			artifacts, err = filter.DoFilterArtifacts(artifacts, filters)
			if err != nil {
				return nil, err
			}
			if len(artifacts) == 0 {
				continue
			}

			for _, artifact := range artifacts {
				resources = append(resources, &model.Resource{
					Type:     model.ResourceTypeCsar,
					Registry: a.Registry,
					Metadata: &model.ResourceMetadata{
						Repository: &model.Repository{
							//test/httpbin
							Name: repository.Name,
							//test.Metadata
							Metadata: project.Metadata,
						},
						Artifacts: []*model.Artifact{artifact},
					},
				})
			}
		}
	}
	return resources, nil
}
func (a *Adapter) CsarExist(name string) (bool, error) {
	//name=project/repo
	_, err := a.getCsarInfo(name)
	if err == nil {
		return true, nil
	}
	if httpErr, ok := err.(*common_http.Error); ok && httpErr.Code == http.StatusNotFound {
		return false, nil
	}
	return false, err
}
func (a *Adapter) DownloadCsar(name string) (io.ReadCloser, error) {
	info, err := a.getCsarInfo(name)
	if err != nil {
		return nil, err
	}
	if info.Version == "" {
		return nil, fmt.Errorf("cannot got the download url for csar %s:%s", name)
	}
	//todo get download url
	project, csarName := utils.ParseRepository(name)
	urlStr := fmt.Sprintf("%s/api/csarrepo/%s/csars/%s/download", a.Client.GetURL(), project, csarName)
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to download the csar %s: %d %s", req.URL.String(), resp.StatusCode, string(body))
	}
	return resp.Body, nil
}
func (a *Adapter) UploadCsar(name string, csar io.Reader) error {
	project, csarName := utils.ParseRepository(name)
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	//todo  define csar suffix
	fw, err := w.CreateFormFile("csar", csarName+".tgz")
	if err != nil {
		return err
	}
	if _, err = io.Copy(fw, csar); err != nil {
		return err
	}
	w.Close()
	//upload to client registry
	url := fmt.Sprintf("%s/api/csarrepo/%s/csars", a.Client.GetURL(), project)
	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &common_http.Error{
			Code:    resp.StatusCode,
			Message: string(data),
		}
	}
	return nil
}
func (a *Adapter) DeleteCsar(name string) error {
	project, csarName := utils.ParseRepository(name)
	url := fmt.Sprintf("%s/api/csarrepo/%s/csars/%s/%s", a.Client.GetURL(), project, csarName)
	return a.httpClient.Delete(url)
}

func (a *Adapter) getCsarInfo(name string) (*csarDetail, error) {
	project, csarName := utils.ParseRepository(name)
	url := fmt.Sprintf("%s/api/csarrepo/%s/csars/%s", a.Client.GetURL(), project, csarName)
	info := &csarDetail{}
	if err := a.httpClient.Get(url, info); err != nil {
		return nil, err
	}
	return info, nil
}
