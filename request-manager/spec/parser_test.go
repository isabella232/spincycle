// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"strings"
	"testing"

	"github.com/go-test/deep"

	"github.com/square/spincycle/v2/test"
)

func TestParseSpec(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	_, err := ParseSpec(sequencesFile, t.Logf)
	if err != nil {
		t.Errorf("failed to parse decomm.yaml, expected success")
	}
}

func TestFailParseSpec(t *testing.T) {
	sequencesFile := specsDir + "fail-parse-spec.yaml" // mistmatched type
	_, err := ParseSpec(sequencesFile, t.Logf)
	if err == nil {
		t.Errorf("unmarshaled string into uint")
	} else {
		switch err.(type) {
		case *yaml.TypeError:
			t.Log(err.Error())
		default:
			t.Errorf("expected yaml.TypeError, got %T: %s", err, err)
		}
	}
}

func TestWarnParseSpec(t *testing.T) {
	sequencesFile := specsDir + "warn-parse-spec.yaml" // duplicated field

	var warning string
	logFunc := func(s string, args ...interface{}) { warning = fmt.Sprintf(s, args...) }

	ParseSpec(sequencesFile, logFunc)
	if warning == "" {
		t.Errorf("failed to give warning for duplicated field")
	} else if strings.Contains(strings.ToLower(warning), "warning") {
		t.Log(warning)
	} else {
		t.Errorf("expected warning containing 'warning' as substring, got: %s", warning)
	}
}

func TestParseSpecsDir(t *testing.T) {
	specsDir := specsDir + "parse-specs-dir"
	_, err := ParseSpecsDir(specsDir, t.Logf)
	if err != nil {
		t.Errorf("failed to parse specs directory, expected success: %s", err)
	}
}

func TestFailParseSpecsDir(t *testing.T) {
	specsDir := specsDir + "fail-parse-specs-dir"
	_, err := ParseSpecsDir(specsDir, t.Logf)
	if err == nil {
		t.Fatalf("successfully parsed specs directory with repeated sequences, expected failure")
	}
	t.Log(err)
}

func TestProcessSpecs(t *testing.T) {
	requiredA := "required-a"
	optionalA := "optional-a"
	staticA := "static-a"

	job := "job"
	jobTypeA := "job-type-a"
	argA := "arg-a"
	argB := "arg-b"

	sequence := "sequence"
	seqB := "seq-b"
	argB0 := "arg-b0"
	argC0 := "arg-c0"
	argC := "arg-c"

	conditional := "conditional"
	seqC := "seq-c"
	argIf := "arg-if"
	argD := "arg-d"
	argD0 := "arg-d0"
	argE := "arg-e"
	argF0 := "arg-f0"
	argF := "arg-f"
	argG := "arg-g"

	specs := Specs{
		Sequences: map[string]*Sequence{
			//   seq-a:
			//     request: true
			//     args:
			//       required:
			//         - name: required-a
			//       optional:
			//         - name: optional-a
			//           default: value
			//       static:
			//         - name: static-a
			//           default: value
			"seq-a": &Sequence{
				Name:    "",
				Request: true,
				Args: SequenceArgs{
					Required: []*Arg{&Arg{Name: &requiredA}},
					Optional: []*Arg{&Arg{Name: &optionalA, Default: &value}},
					Static:   []*Arg{&Arg{Name: &staticA, Default: &value}},
				},
				Nodes: map[string]*Node{
					//       node-a:
					//         category: job
					//         type: job-type-a
					//         each:
					//           - list:element
					//         args:
					//           - expected: arg-a
					//         sets:
					//           - arg: arg-b
					//         deps: []
					//         retry: 3
					"node-a": &Node{
						Name:         "",
						Category:     &job,
						NodeType:     &jobTypeA,
						Each:         []string{"list:element"},
						Args:         []*NodeArg{&NodeArg{Expected: &argA}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argB}},
						Dependencies: []string{},
						Retry:        3,
					},
					//       node-b:
					//         category: sequence
					//         type: seq-b
					//         each:
					//           - list:element
					//         args:
					//           - expected: arg-b
					//             given: arg-b0
					//         sets:
					//           - arg: arg-c0
					//             as: arg-c
					//         deps: [node-1]
					//         retry: 3
					//         retryWait: 10s
					"node-b": &Node{
						Name:         "",
						Category:     &sequence,
						NodeType:     &seqB,
						Each:         []string{"list:element"},
						Args:         []*NodeArg{&NodeArg{Expected: &argB, Given: &argB0}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argC0, As: &argC}},
						Dependencies: []string{"node-1"},
						Retry:        3,
						RetryWait:    "10s",
					},
					//       node-c:
					//         category: conditional:
					//         type: seq-c
					//         if: arg-if
					//         eq:
					//           cond-1: seq-1
					//           default: default
					//         args:
					//           - expected: arg-d
					//             given: arg-d0
					//           - expected: arg-e
					//         sets:
					//           - arg: arg-f0
					//             as: arg-f
					//           - arg: arg-g
					//         deps: [node-1]
					"node-c": &Node{
						Name:         "",
						Category:     &conditional,
						NodeType:     &seqC,
						If:           &argIf,
						Eq:           map[string]string{"cond-1": "seq-1", "default": "default"},
						Args:         []*NodeArg{&NodeArg{Expected: &argD, Given: &argD0}, &NodeArg{Expected: &argE}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argF0, As: &argF}, &NodeSet{Arg: &argG}},
						Dependencies: []string{"node-1"},
					},
				},
			},
		},
	}

	err := ProcessSpecs(specs)
	if err != nil {
		t.Fatalf("Error processing specs: %s", err)
	}

	expectedSpecs := Specs{
		Sequences: map[string]*Sequence{
			// 'Name' should be set
			"seq-a": &Sequence{
				Name:    "seq-a",
				Request: true,
				Args: SequenceArgs{
					Required: []*Arg{&Arg{Name: &requiredA}},
					Optional: []*Arg{&Arg{Name: &optionalA, Default: &value}},
					Static:   []*Arg{&Arg{Name: &staticA, Default: &value}},
				},

				Nodes: map[string]*Node{
					// 'Name', 'Given', 'As', and 'RetryWait' should've been set
					"node-a": &Node{
						Name:         "node-a",
						Category:     &job,
						NodeType:     &jobTypeA,
						Each:         []string{"list:element"},
						Args:         []*NodeArg{&NodeArg{Expected: &argA, Given: &argA}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argB, As: &argB}},
						Dependencies: []string{},
						Retry:        3,
						RetryWait:    "0s",
					},
					// 'Name' should've been set, everything else stays the same
					"node-b": &Node{
						Name:         "node-b",
						Category:     &sequence,
						NodeType:     &seqB,
						Each:         []string{"list:element"},
						Args:         []*NodeArg{&NodeArg{Expected: &argB, Given: &argB0}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argC0, As: &argC}},
						Dependencies: []string{"node-1"},
						Retry:        3,
						RetryWait:    "10s",
					},
					// 'Name', 'Given' for arg-e, and 'As' for arg-g should've been set
					"node-c": &Node{
						Name:         "node-c",
						Category:     &conditional,
						NodeType:     &seqC,
						If:           &argIf,
						Eq:           map[string]string{"cond-1": "seq-1", "default": "default"},
						Args:         []*NodeArg{&NodeArg{Expected: &argD, Given: &argD0}, &NodeArg{Expected: &argE, Given: &argE}},
						Sets:         []*NodeSet{&NodeSet{Arg: &argF0, As: &argF}, &NodeSet{Arg: &argG, As: &argG}},
						Dependencies: []string{"node-1"},
					},
				},
			},
		},
	}

	if diff := deep.Equal(specs, expectedSpecs); diff != nil {
		test.Dump(specs)
		t.Error(diff)
	}
}
