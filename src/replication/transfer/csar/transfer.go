/*
@Time : 2021/6/10
@Author : jzd
@Project: harbor
*/
package csar

import (
	"errors"
	"github.com/goharbor/harbor/src/lib/log"
	"github.com/goharbor/harbor/src/replication/adapter"
	"github.com/goharbor/harbor/src/replication/model"
	trans "github.com/goharbor/harbor/src/replication/transfer"
)

func init() {
	if err := trans.RegisterFactory(model.ResourceTypeCsar, factory); err != nil {
		log.Errorf("failed to register transfer factory: %v", err)
	}
}

func factory(logger trans.Logger, stopFunc trans.StopFunc) (trans.Transfer, error) {
	return &transfer{
		logger:    logger,
		isStopped: stopFunc,
	}, nil
}

type transfer struct {
	logger    trans.Logger
	isStopped trans.StopFunc
	src       adapter.CsarRegistry
	dst       adapter.CsarRegistry
}

func (t *transfer) Transfer(src *model.Resource, dst *model.Resource) error {
	// initialize
	if err := t.initialize(src, dst); err != nil {
		return err
	}

	// delete the chart on destination registry
	if dst.Deleted {
		//name like test/httpbin
		return t.delete(dst.Metadata.Repository.Name)
	}
	// copy the csar from source registry to the destination
	t.logger.Infof("copy the csar %v from source registry to the destination csar %v", src.Metadata.Repository.Name, dst.Metadata.Repository.Name)
	return t.copy(src.Metadata.Repository.Name, dst.Metadata.Repository.Name, dst.Override)
}

func (t *transfer) initialize(src, dst *model.Resource) error {
	// create client for source registry
	srcReg, err := createRegistry(src.Registry)
	if err != nil {
		t.logger.Errorf("failed to create client for source registry: %v", err)
		return err
	}
	t.src = srcReg
	t.logger.Infof("client for source registry [type: %s, URL: %s, insecure: %v] created",
		src.Registry.Type, src.Registry.URL, src.Registry.Insecure)

	// create client for destination registry
	dstReg, err := createRegistry(dst.Registry)
	if err != nil {
		t.logger.Errorf("failed to create client for destination registry: %v", err)
		return err
	}
	t.dst = dstReg
	t.logger.Infof("client for destination registry [type: %s, URL: %s, insecure: %v] created",
		dst.Registry.Type, dst.Registry.URL, dst.Registry.Insecure)

	return nil
}

//harbor registry
func createRegistry(reg *model.Registry) (adapter.CsarRegistry, error) {
	//reg.Type=harbor
	factory, err := adapter.GetFactory(reg.Type)
	if err != nil {
		return nil, err
	}
	ad, err := factory.Create(reg)
	if err != nil {
		return nil, err
	}
	//csar adapter impl Adapter interface
	registry, ok := ad.(adapter.CsarRegistry)
	if !ok {
		return nil, errors.New("the adapter doesn't implement the \"CsarRegistry\" interface")
	}
	return registry, nil
}

func (t *transfer) delete(name string) error {
	exist, err := t.dst.CsarExist(name)
	if err != nil {
		t.logger.Errorf("failed to check the existence of csar %s on the destination registry: %v", name, err)
		return err
	}
	if !exist {
		t.logger.Infof("the csar %s doesn't exist on the destination registry, skip",
			name)
		return nil
	}
	t.logger.Infof("deleting the csar %s on the destination registry...", name)
	if err := t.dst.DeleteCsar(name); err != nil {
		t.logger.Errorf("failed to delete the csar %s on the destination registry: %v", name, err)
		return err
	}
	t.logger.Infof("delete the csar name on the destination registry completed", name)
	return nil
}

func (t *transfer) shouldStop() bool {
	isStopped := t.isStopped()
	if isStopped {
		t.logger.Info("the job is stopped")
	}
	return isStopped
}

func (t *transfer) copy(src, dst string, override bool) error {
	if t.shouldStop() {
		return nil
	}
	t.logger.Infof("copying %s(source registry) to %s(destination registry)...",
		src, dst)
	// check the existence of the csar on the destination registry
	exist, err := t.dst.CsarExist(dst)
	if err != nil {
		t.logger.Errorf("failed to check the existence of csar %s on the destination registry: %v", dst, err)
		return err
	}
	if exist {
		// the same name csar exists, but not allowed to override
		if !override {
			t.logger.Warningf("the same name csar %s exists on the destination registry, but the \"override\" is set to false, skip",
				dst)
			return nil
		}
		// the same name csar exists, but allowed to override
		t.logger.Warningf("the same name csar %s exists on the destination registry and the \"override\" is set to true, continue...",
			dst)
	}
	// copy the chart between the source and destination registries
	csar, err2 := t.src.DownloadCsar(src)
	if err2 != nil {
		t.logger.Errorf("failed to download the csar %s %v", src, err)
		return err2
	}
	defer csar.Close()
	if err = t.dst.UploadCsar(dst, csar); err != nil {
		t.logger.Errorf("failed to upload the csar %s:%s: %v", csar, err)
		return err
	}
	t.logger.Infof("copy %s(source registry) to %s(destination registry) completed",
		src, dst)
	return nil
}
