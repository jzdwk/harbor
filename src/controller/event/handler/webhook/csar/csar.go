/*
@Time : 2021/7/5
@Author : jzd
@Project: harbor
*/
package csar

import (
	"context"
	"errors"
	"fmt"
	"github.com/goharbor/harbor/src/common/models"
	"github.com/goharbor/harbor/src/controller/event"
	"github.com/goharbor/harbor/src/controller/event/handler/util"
	"github.com/goharbor/harbor/src/controller/project"
	"github.com/goharbor/harbor/src/core/config"
	"github.com/goharbor/harbor/src/lib/log"
	"github.com/goharbor/harbor/src/pkg/notification"
	"github.com/goharbor/harbor/src/pkg/notifier/model"
)

// Handler preprocess chart event data
type Handler struct {
	Context func() context.Context
}

// Name ...
func (csh *Handler) Name() string {
	return "CsarWebhook"
}

// Handle preprocess csar event data and then publish hook event
func (csh *Handler) Handle(value interface{}) error {
	csarEvent, ok := value.(*event.CsarEvent)
	if !ok {
		return errors.New("invalid csar event type")
	}

	if csarEvent == nil || len(csarEvent.ProjectName) == 0 || len(csarEvent.CsarName) == 0 {
		return fmt.Errorf("data miss in csar event: %v", csarEvent)
	}

	prj, err := project.Ctl.Get(csh.Context(), csarEvent.ProjectName, project.Metadata(true))
	if err != nil {
		log.Errorf("failed to find project[%s] for chart event: %v", csarEvent.ProjectName, err)
		return err
	}
	//get webhook policy from related project
	policies, err := notification.PolicyMgr.GetRelatedPolices(prj.ProjectID, csarEvent.EventType)
	if err != nil {
		log.Errorf("failed to find policy for %s event: %v", csarEvent.EventType, err)
		return err
	}
	// if cannot find policy including event type in project, return directly
	if len(policies) == 0 {
		log.Debugf("cannot find policy for %s event: %v", csarEvent.EventType, csarEvent)
		return nil
	}

	payload, err := constructCsarPayload(csarEvent, prj)
	if err != nil {
		return err
	}

	err = util.SendHookWithPolicies(policies, payload, csarEvent.EventType)
	if err != nil {
		return err
	}

	return nil
}

// IsStateful ...
func (csh *Handler) IsStateful() bool {
	return false
}

func constructCsarPayload(event *event.CsarEvent, project *models.Project) (*model.Payload, error) {
	repoType := models.ProjectPrivate
	if project.IsPublic() {
		repoType = models.ProjectPublic
	}
	payload := &model.Payload{
		Type:    event.EventType,
		OccurAt: event.OccurAt.Unix(),
		EventData: &model.EventData{
			Repository: &model.Repository{
				Name:         event.CsarName,
				Namespace:    event.ProjectName,
				RepoFullName: fmt.Sprintf("%s/%s", event.ProjectName, event.CsarName),
				RepoType:     repoType,
			},
		},
		Operator: event.Operator,
	}
	extURL, err := config.ExtEndpoint()
	if err != nil {
		return nil, fmt.Errorf("get external endpoint failed: %v", err)
	}
	resourcePrefix := fmt.Sprintf("/api/%s/csarrepo/%s/csars/%s", extURL, event.ProjectName, event.CsarName)
	resource := &model.Resource{
		ResourceURL: resourcePrefix,
	}
	payload.EventData.Resources = append(payload.EventData.Resources, resource)

	return payload, nil
}
