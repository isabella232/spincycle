// Copyright 2017-2019, Square, Inc.

package cmd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
	"github.com/square/spincycle/spinc/config"
	"github.com/square/spincycle/test/mock"
)

var args []proto.RequestArg = []proto.RequestArg{
	{
		Name:  "key",
		Type:  proto.ARG_TYPE_REQUIRED,
		Value: "value",
	},
	{
		Name:  "key2",
		Type:  proto.ARG_TYPE_REQUIRED,
		Value: "val2",
	},
	{
		Name:  "opt",
		Type:  proto.ARG_TYPE_OPTIONAL,
		Value: "not-shown",
	},
}

func TestPs(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				Status:    "jobstatus",
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "requestname",
				User:         "owner",
				FinishedJobs: 1,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	rmc := &mock.RMClient{
		SysStatRunningFunc: func() (proto.RunningStatus, error) {
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options: config.Options{
			Verbose: false,
		},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	expectOutput := `ID                  	   N	NJOBS	  TIME	OWNER	JOB	
b9uvdi8tk9kahl8ppvbg	   1	    9	   3.0	owner	jobname
`

	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestPsVerbose(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				Status:    "jobstatus",
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "requestname",
				User:         "owner",
				FinishedJobs: 2,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	rmc := &mock.RMClient{
		SysStatRunningFunc: func() (proto.RunningStatus, error) {
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options: config.Options{
			Verbose: true,
		},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	// There's a trailing space after "val2 ". Only required args in list order
	// because that matches spec order.
	expectOutput := `ID                  	   N	NJOBS	  TIME	OWNER	JOB	REQUEST
b9uvdi8tk9kahl8ppvbg	   2	    9	   3.0	owner	jobname	requestname  key=value key2=val2 
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}
