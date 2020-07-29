// Copyright 2017-2020, Square, Inc.

package template

import (
	"fmt"
	"strings"

	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

const DEFAULT = "default"

// Contains specs to construct template graphs and an ID generator provided by caller.
// Also contains map of all (successfully constructed) templates and map of fatal errors
// occurring during construction. Membership in map of templates and map of errors is mutually
// exclusive.
type Grapher struct {
	// user-provided
	AllSequences map[string]*spec.SequenceSpec // All sequences that were read in from spec.Specs
	IdGenFactory id.GeneratorFactory           // Generates per-template unique IDs for nodes
	LogFunc      func(string, ...interface{})  // Printf-like function to log errors and warnings

	// filled in while creating templates
	SequenceTemplates map[string]*Graph // Template graphs for (valid) sequences in AllSequences
	SequenceErrors    map[string]error  // Error generated by invalid sequence, if any (sequence name -> error)
}

func NewGrapher(specs spec.Specs, idGenFactory id.GeneratorFactory, logFunc func(string, ...interface{})) *Grapher {
	return &Grapher{
		AllSequences:      specs.Sequences,
		IdGenFactory:      idGenFactory,
		LogFunc:           logFunc,
		SequenceTemplates: make(map[string]*Graph),
		SequenceErrors:    make(map[string]error),
	}
}

// Create all templates. Returns an error if any error occurs. Specifics of errors are
// recorded by log function.
func (o *Grapher) CreateTemplates() error {
	errOccurred := false
	for requestName, _ := range o.AllSequences {
		err := o.buildSequence(requestName)
		if err != nil {
			errOccurred = true
		}
	}
	if errOccurred {
		return fmt.Errorf("error occurred while creating templates")
	}
	return nil
}

// Builds sequence. Returns an error if sequence is invalid.
// This should be the ONLY grapher function to log errors using `LogFunc` and to modify `Sequence(Templates|Errors)`
// for the sake of preserving sanity.
func (o *Grapher) buildSequence(sequenceName string) error {
	seq, ok := o.AllSequences[sequenceName]
	if !ok { // this shouldn't happen, because there should be a static check ensuring that all sequences actually exist
		err := fmt.Errorf("cannot find definition; this is a bug in the code")
		o.LogFunc("error: sequence %s: %s\n", sequenceName, err)
		o.SequenceErrors[sequenceName] = err
		return err
	}

	/* Check if we've already built (or tried to build) this sequence. */
	_, ok = o.SequenceTemplates[sequenceName]
	if ok {
		return nil
	}
	err, ok := o.SequenceErrors[sequenceName]
	if ok {
		return err
	}

	/* Build all subsequences, including those listed in conditional nodes. */
	/* Check that subsequences set args they're supposed to. */
	subsequences := o.getSubsequences(seq) // node name --> list of subsequence names
	err = o.buildAllSubsequences(subsequences)
	if err != nil {
		err := fmt.Errorf("failed to build subsequence(s)")
		o.SequenceErrors[sequenceName] = err
		return err
	}
	sets := o.getActualSets(subsequences)          // node name --> args actually set
	missingSets := getMissingSets(seq.Nodes, sets) // node name --> args declared in `sets` that weren't actually set
	if len(missingSets) > 0 {
		err := fmt.Errorf("one or more nodes did not actually set job args declared in `sets`")
		for nodeName, missing := range missingSets {
			o.LogFunc("error: sequence %s, node %s: job arg(s) %s listed in `sets` were not actually set\n", sequenceName, nodeName, strings.Join(missing, ", "))
		}
		o.SequenceErrors[sequenceName] = err
		return err
	}

	/* Build template. */
	jobArgs := getAllSequenceArgs(seq) // map of sequence arg (including optional+defualt) --> true
	template, err := o.getTemplate(seq, jobArgs)
	if err != nil {
		o.LogFunc("error: sequence %s: %s\n", sequenceName, err)
		o.SequenceErrors[sequenceName] = err
		return err
	}
	o.SequenceTemplates[sequenceName] = template

	return nil
}

// Get all subsequences as a map of node name --> list of subsequences
// List of subsequences may contain duplicates
func (o *Grapher) getSubsequences(seq *spec.SequenceSpec) map[string][]string {
	subsequences := map[string][]string{}
	for nodeName, node := range seq.Nodes {
		subseq := []string{}
		if node.IsSequence() {
			subseq = append(subseq, *node.NodeType)
		} else if node.IsConditional() {
			for _, seq := range node.Eq {
				// don't add it if it's a job
				if _, ok := o.AllSequences[seq]; ok {
					subseq = append(subseq, seq)
				}
			}
		}
		if len(subseq) > 0 {
			subsequences[nodeName] = subseq
		}
	}
	return subsequences
}

// Build all subsequences in `subsequences`, a map of node name --> list of subsequences
// Returns error if any subsequence fails to build
func (o *Grapher) buildAllSubsequences(subsequences map[string][]string) error {
	errOccurred := false
	for _, subseqs := range subsequences {
		for _, seq := range subseqs {
			err := o.buildSequence(seq)
			if err != nil {
				errOccurred = true
			}
		}
	}
	if errOccurred {
		return fmt.Errorf("error building subsequence")
	}
	return nil
}

// Get the job args that were actually set by subsequences as a map of node name --> (intersection) of set of output job args
func (o *Grapher) getActualSets(subsequences map[string][]string) map[string]map[string]bool {
	actualSets := map[string]map[string]bool{}
	for nodeName, subseqs := range subsequences {
		actualMap := map[string]int{} // output job arg --> # of subsequences that output it
		for _, seq := range subseqs {
			if template, ok := o.SequenceTemplates[seq]; ok {
				for arg, _ := range template.Sets {
					actualMap[arg]++
				}
			}
		}

		actualSets[nodeName] = map[string]bool{}
		for seq, count := range actualMap {
			if count == len(subseqs) {
				actualSets[nodeName][seq] = true
			}
		}
	}
	return actualSets
}

// Get job args declared in `sets` that weren't actually set as map of node name --> list of missing job args
// Assumes all node names in `actualSets` appears in `nodes`
func getMissingSets(nodes map[string]*spec.NodeSpec, actualSets map[string]map[string]bool) map[string][]string {
	missingSets := map[string][]string{}
	for nodeName, sets := range actualSets {
		nodeSpec, _ := nodes[nodeName]
		missing := []string{}
		for _, nodeSet := range nodeSpec.Sets {
			if !sets[*nodeSet.Arg] {
				missing = append(missing, *nodeSet.Arg)
			}
		}
		if len(missing) > 0 {
			missingSets[nodeName] = missing
		}
	}
	return missingSets
}

// Get the minimal set of job args that the sequence starts with
// In the context of a wider request of which this sequence is a part, there may be more job
// args available, but this sequence should not access them, so they are irrelevant for our purposes
func getAllSequenceArgs(seq *spec.SequenceSpec) map[string]bool {
	jobArgs := map[string]bool{}
	for _, arg := range seq.Args.Required {
		jobArgs[*arg.Name] = true
	}
	for _, arg := range seq.Args.Optional {
		jobArgs[*arg.Name] = true
	}
	for _, arg := range seq.Args.Static {
		jobArgs[*arg.Name] = true
	}
	return jobArgs
}

// Creates the actual template. `jobArgs` are the set of job args that the sequence starts
// out with, presumably the sequence args.
func (o *Grapher) getTemplate(seq *spec.SequenceSpec, jobArgs map[string]bool) (*Graph, error) {
	// Generates IDs unique within the template
	idgen := o.IdGenFactory.Make()

	template, err := newGraph(seq.Name, idgen)
	if err != nil {
		return nil, err
	}
	g := &template.Graph

	nodes := seq.Nodes

	components := map[*spec.NodeSpec]*Node{}
	for _, node := range nodes {
		components[node], err = template.NewNode(node)
		if err != nil {
			return nil, err
		}
	}

	componentsToAdd := map[*spec.NodeSpec]*Node{}
	for k, v := range components {
		componentsToAdd[k] = v
	}
	componentsAdded := map[*spec.NodeSpec]bool{}

	for len(componentsToAdd) > 0 {
		// Build graph by adding components, starting from the source node, and then
		// adding all adjacent nodes to the source node, and so on.
		// We cannot add components in any order because we do not know the reverse dependencies
		componentAdded := false
		for node, component := range componentsToAdd {
			if containsAll(componentsAdded, node.Dependencies, nodes) {
				missingArgs, err := getMissingArgs(node, jobArgs)
				if err != nil {
					return nil, err
				}
				if len(missingArgs) > 0 {
					return nil, fmt.Errorf("node %s missing job args: %s", node.Name, strings.Join(missingArgs, ", "))
				}

				if len(node.Dependencies) == 0 {
					// If there are no dependencies, then this job will come "first". Insert it
					// directly after the Start node.
					err := template.AddNodeAfter(component, g.First)
					if err != nil {
						return nil, err
					}
				} else {
					// If all the dependencies for this job have been added to the graph,
					// then add it. If not all the dependecies have been added, skip it for now.

					// Insert the component between all its dependencies and the end node.
					for _, dependencyName := range node.Dependencies {
						dependency := nodes[dependencyName]
						prevComponent := components[dependency]
						err := template.AddNodeAfter(component, prevComponent)
						if err != nil {
							return nil, err
						}
					}
				}

				// update job args map
				for _, nodeSet := range node.Sets {
					jobArgs[*nodeSet.As] = true
				}

				// remove this node from the components to add list
				delete(componentsToAdd, node)
				componentsAdded[node] = true

				componentAdded = true
			}

		}

		/* Ensure there are no circular dependencies. */
		// If we were unable to add nodes, there must be a circular dependency
		if !componentAdded {
			cs := []string{}
			for c, _ := range componentsToAdd {
				cs = append(cs, c.Name)
			}
			return nil, fmt.Errorf("impossible dependencies found amongst: %v", cs)
		}
	}

	// Assert g is a well formed graph
	if !g.IsValidGraph() {
		return nil, fmt.Errorf("malformed graph created")
	}

	template.Sets = jobArgs

	return template, nil
}

// containsAll is a convenience function for checking membership in a map.
// Returns true if m contains every elements in ss
func containsAll(m map[*spec.NodeSpec]bool, ss []string, nodes map[string]*spec.NodeSpec) bool {
	for _, s := range ss {
		name := nodes[s]
		if _, ok := m[name]; !ok {
			return false
		}
	}
	return true
}

// Returns a list of node args that aren't present in the job args map.
func getMissingArgs(n *spec.NodeSpec, jobArgs map[string]bool) ([]string, error) {
	missing := []string{}

	// Assert that the iterable variable is present.
	for _, each := range n.Each {
		if each == "" {
			continue
		}
		if len(strings.Split(each, ":")) != 2 { // this is malformed input
			return missing, fmt.Errorf("in node %s: malformed input to `each:`", n.Name)
		}
		iterateSet := strings.Split(each, ":")[0]
		if !jobArgs[iterateSet] {
			missing = append(missing, iterateSet)
		}
	}

	// Assert that the conditional variable is present.
	if n.If != nil {
		if !jobArgs[*n.If] {
			missing = append(missing, *n.If)
		}
	}

	// Assert all other defined args are present
	for _, arg := range n.Args {
		if !jobArgs[*arg.Given] {
			missing = append(missing, *arg.Given)
		}
	}

	return missing, nil
}
