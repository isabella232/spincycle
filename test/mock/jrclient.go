// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"net/url"

	"github.com/square/spincycle/v2/proto"
)

var (
	ErrJRClient = errors.New("forced error in jr client")
)

type JRClient struct {
	NewJobChainFunc    func(string, proto.JobChain) (*url.URL, error)
	ResumeJobChainFunc func(string, proto.SuspendedJobChain) (*url.URL, error)
	StartRequestFunc   func(string, string) error
	StopRequestFunc    func(string, string) error
	RunningFunc        func(string, proto.StatusFilter) ([]proto.JobStatus, error)
}

func (c *JRClient) NewJobChain(baseURL string, jc proto.JobChain) (*url.URL, error) {
	if c.NewJobChainFunc != nil {
		return c.NewJobChainFunc(baseURL, jc)
	}
	return nil, nil
}

func (c *JRClient) ResumeJobChain(baseURL string, sjc proto.SuspendedJobChain) (*url.URL, error) {
	if c.ResumeJobChainFunc != nil {
		return c.ResumeJobChainFunc(baseURL, sjc)
	}
	return nil, nil
}

func (c *JRClient) StartRequest(baseURL string, requestId string) error {
	if c.StartRequestFunc != nil {
		return c.StartRequestFunc(baseURL, requestId)
	}
	return nil
}

func (c *JRClient) StopRequest(baseURL string, requestId string) error {
	if c.StopRequestFunc != nil {
		return c.StopRequestFunc(baseURL, requestId)
	}
	return nil
}

func (c *JRClient) Running(baseURL string, f proto.StatusFilter) ([]proto.JobStatus, error) {
	if c.RunningFunc != nil {
		return c.RunningFunc(baseURL, f)
	}
	return []proto.JobStatus{}, nil
}
