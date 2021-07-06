/*
@Time : 2021/7/6
@Author : jzd
@Project: harbor
*/
package metadata

import (
	event2 "github.com/goharbor/harbor/src/controller/event"
	"github.com/goharbor/harbor/src/pkg/notifier/event"
	"time"
)

// CsarMetaData defines meta data of chart event
type CsarMetaData struct {
	ProjectName string
	CsarName    string
	//todo does csar version necessary
	//Versions    []string
	OccurAt  time.Time
	Operator string
}

func (cmd *CsarMetaData) convert(evt *event2.CsarEvent) {
	evt.ProjectName = cmd.ProjectName
	evt.OccurAt = cmd.OccurAt
	evt.Operator = cmd.Operator
	evt.CsarName = cmd.CsarName
	//evt.Versions = cmd.Versions
}

// CsarUploadMetaData defines meta data of chart upload event
type CsarUploadMetaData struct {
	CsarMetaData
}

// Resolve chart uploading metadata into common chart event
func (cu *CsarUploadMetaData) Resolve(event *event.Event) error {
	data := &event2.CsarEvent{
		EventType: event2.TopicUploadCsar,
	}
	cu.convert(data)

	event.Topic = event2.TopicUploadCsar
	event.Data = data
	return nil
}

// CsarDownloadMetaData defines meta data of chart download event
type CsarDownloadMetaData struct {
	CsarMetaData
}

// Resolve chart download metadata into common chart event
func (cd *CsarDownloadMetaData) Resolve(evt *event.Event) error {
	data := &event2.CsarEvent{
		EventType: event2.TopicDownloadCsar,
	}
	cd.convert(data)

	evt.Topic = event2.TopicDownloadCsar
	evt.Data = data
	return nil
}

// CsarDeleteMetaData defines meta data of chart delete event
type CsarDeleteMetaData struct {
	CsarMetaData
}

// Resolve chart delete metadata into common chart event
func (cd *CsarDeleteMetaData) Resolve(evt *event.Event) error {
	data := &event2.CsarEvent{
		EventType: event2.TopicDeleteCsar,
	}
	cd.convert(data)

	evt.Topic = event2.TopicDeleteCsar
	evt.Data = data
	return nil
}
