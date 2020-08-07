// Copyright 2017-2020, Square, Inc.

package template

import (
	"fmt"
	"strings"

	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

const DEFAULT = "default"

// Construct template graphs using specs and an ID generator provided by caller.
// Performs graph checks while constructing graphs and logs errors that occur.
// Template graphs are later used to create job chains.
type Grapher struct {
	// User-provided
	allSequences map[string]*spec.Sequence    // All sequences read in from request specs
	idGenFactory id.GeneratorFactory          // Generate per-template unique IDs for nodes
	logFunc      func(string, ...interface{}) // Printf-like function to log errors and warnings

	// Filled in while creating templates
	// Membership in these maps is mutually exclusive
	sequenceTemplates map[string]*Graph // Template graphs for (valid) sequences in allSequences (sequence name -> tempalte)
	sequenceErrors    map[string]error  // Error generated by invalid sequence(s), if any (sequence name -> error)
}

func NewGrapher(specs spec.Specs, idGenFactory id.GeneratorFactory, logFunc func(string, ...interface{})) *Grapher {
	return &Grapher{
		allSequences:      specs.Sequences,
		idGenFactory:      idGenFactory,
		logFunc:           logFunc,
		sequenceTemplates: nil, // Leave this nil to indicate that we have no templates yet.
		sequenceErrors:    make(map[string]error),
	}
}

// Create all templates. Returns templates and a bool indicating whether an error occurred.
func (o *Grapher) CreateTemplates() (map[string]*Graph, bool) {
	if o.sequenceTemplates != nil {
		return o.sequenceTemplates, len(o.sequenceErrors) == 0
	}

	o.sequenceTemplates = map[string]*Graph{}
	ok := true
	for sequenceName, _ := range o.allSequences {
		build := o.buildSequence(sequenceName)
		ok = ok && build
	}
	return o.sequenceTemplates, ok
}

// Builds sequence. Returns true iff build succeeds.
// This should be the ONLY grapher function to log errors using `logFunc` and to modify `sequence(Templates|Errors)`
// for the sake of preserving sanity.
func (o *Grapher) buildSequence(sequenceName string) bool {
	/* Check if we've already built (or tried to build) this sequence. */
	_, ok := o.sequenceTemplates[sequenceName]
	if ok {
		return true
	}
	_, ok = o.sequenceErrors[sequenceName]
	if ok {
		return false
	}

	/* retErr and template are both nil. They should only be set right before a return. */
	var retErr error
	var template *Graph
	defer func() {
		if retErr != nil {
			o.logFunc("error: sequence %s: %s", sequenceName, retErr)
			o.sequenceErrors[sequenceName] = retErr
		}
		if template != nil {
			o.sequenceTemplates[sequenceName] = template
		}
	}()

	/* Build all subsequences, including those listed in conditional nodes. */
	seq, ok := o.allSequences[sequenceName]
	if !ok { // this shouldn't happen, because there should be a static check ensuring that all sequences actually exist
		retErr = fmt.Errorf("cannot find definition")
		return false
	}
	subsequences := o.getSubsequences(seq) // node name --> list of subsequence names
	ok = o.buildAllSubsequences(subsequences)
	if !ok {
		retErr = fmt.Errorf("failed to build subsequence(s)")
		return false
	}

	/* Check that subsequences set args they're supposed to. */
	sets := o.getActualSets(subsequences)          // node name --> args actually set
	missingSets := getMissingSets(seq.Nodes, sets) // node name --> args declared in `sets` that weren't actually set
	if len(missingSets) > 0 {
		msg := []string{}
		for nodeName, missing := range missingSets {
			msg = append(msg, fmt.Sprintf("%s (failed to set %s)", nodeName, strings.Join(missing, ", ")))
		}
		multiple := ""
		if len(missingSets) > 1 {
			multiple = "s"
		}
		retErr = fmt.Errorf("node%s did not actually set job args declared in 'sets': %s", multiple, strings.Join(msg, "; "))
		return false
	}

	/* Build template. */
	temp, err := o.getTemplate(seq)
	if err != nil {
		retErr = err
		return false
	}
	template = temp
	return true
}

// Get subsequences as a map of node name --> list of subsequences.
func (o *Grapher) getSubsequences(seq *spec.Sequence) map[string][]string {
	subsequences := map[string][]string{}
	for nodeName, node := range seq.Nodes {
		subseq := []string{}
		if node.IsSequence() {
			subseq = append(subseq, *node.NodeType)
		} else if node.IsConditional() {
			for _, seq := range node.Eq {
				// don't add it if it's not a sequence (i.e. a job)
				if _, ok := o.allSequences[seq]; ok {
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

// Build all subsequences in `subsequences`, a map of node name --> list of subsequences.
func (o *Grapher) buildAllSubsequences(subsequences map[string][]string) bool {
	ok := true
	for _, subseqs := range subsequences {
		for _, seq := range subseqs {
			build := o.buildSequence(seq)
			ok = ok && build
		}
	}
	return ok
}

// Get the job args that were actually set by subsequences as a map of node name --> (intersection) of set of output job args.
func (o *Grapher) getActualSets(subsequences map[string][]string) map[string]map[string]bool {
	actualSets := map[string]map[string]bool{}
	for nodeName, nodeSubseqs := range subsequences {
		actualMap := map[string]int{} // output job arg --> # of subsequences that output it
		for _, seq := range nodeSubseqs {
			if template, ok := o.sequenceTemplates[seq]; ok {
				for arg, _ := range template.sets {
					actualMap[arg]++
				}
			}
		}

		actualSets[nodeName] = map[string]bool{}
		for seq, count := range actualMap {
			if count == len(nodeSubseqs) {
				actualSets[nodeName][seq] = true
			}
		}
	}
	return actualSets
}

// Get job args declared in `sets` that weren't actually set as map of node name --> list of missing job args.
// Assumes all node names in `actualSets` appears in `nodes`.
func getMissingSets(nodes map[string]*spec.Node, actualSets map[string]map[string]bool) map[string][]string {
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

// Get the minimal set of job args that the sequence starts with.
// In the context of a wider request of which this sequence is a part, there may be more job
// args available, but this sequence should not access them, so they are irrelevant for our purposes.
func getAllSequenceArgs(seq *spec.Sequence) map[string]bool {
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

// Creates the actual template.
// This function should set all fields in a template.Graph; no other function
// will (or should) perform any modifications on it.
func (o *Grapher) getTemplate(seq *spec.Sequence) (*Graph, error) {
	// Generates IDs unique within the template
	idgen := o.idGenFactory.Make()

	// The graph we'll be filling in
	template, err := newGraph(seq.Name, idgen)
	if err != nil {
		return nil, err
	}

	// Set of job args available, i.e. sequence args + args set by nodes in graph so far
	jobArgs := getAllSequenceArgs(seq)

	// Create a graph node for every node in the spec
	components := map[*spec.Node]*Node{}
	for _, node := range seq.Nodes {
		components[node], err = template.newNode(node)
		if err != nil {
			return nil, err
		}
	}

	// Components we've yet to add
	// It's the complement of the set of nodes in the graph with respect to the set of all nodes
	componentsToAdd := map[*spec.Node]*Node{}
	for k, v := range components {
		componentsToAdd[k] = v
	}
	componentsAdded := map[*spec.Node]bool{}

	// Build graph by adding components, starting from the source node, and then
	// adding all adjacent nodes to the source node, and so on.
	// We cannot add components in any order because we do not know the reverse dependencies
	for len(componentsToAdd) > 0 {

		componentAdded := false

		for node, component := range componentsToAdd {
			if dependenciesSatisfied(componentsAdded, node.Dependencies, seq.Nodes) {
				// Dependencies for node have been satisfied; presumably, all input job args are
				// present in job args map. If not, it's an error.
				missingArgs, err := getMissingArgs(node, jobArgs)
				if err != nil {
					return nil, err
				}
				if len(missingArgs) > 0 {
					return nil, fmt.Errorf("node %s missing job args: %s", node.Name, strings.Join(missingArgs, ", "))
				}

				// Insert component into graph
				if len(node.Dependencies) == 0 {
					// Case: no dependencies; insert directly after start node
					err := template.addNodeAfter(component, template.graph.First)
					if err != nil {
						return nil, err
					}
				} else {
					// Case: dependencies exist; insert between all its dependencies and the end node
					for _, dependencyName := range node.Dependencies {
						dependency := seq.Nodes[dependencyName]
						prevComponent := components[dependency]
						err := template.addNodeAfter(component, prevComponent)
						if err != nil {
							return nil, err
						}
					}
				}

				// Update job args map
				for _, nodeSet := range node.Sets {
					jobArgs[*nodeSet.As] = true
				}

				delete(componentsToAdd, node)
				componentsAdded[node] = true
				componentAdded = true
			}

		}

		// If we were unable to add nodes, there must be a circular dependency, which is an error
		if !componentAdded {
			cs := []string{}
			for c, _ := range componentsToAdd {
				cs = append(cs, c.Name)
			}
			return nil, fmt.Errorf("impossible dependencies found amongst: %v", cs)
		}
	}

	// Make sure our code isn't buggy
	if !template.graph.IsValidGraph() {
		return nil, fmt.Errorf("malformed graph created")
	}

	template.sets = jobArgs
	return template, nil
}

// Check whether the set of nodes in graph (`inGraph`) satisfies all `dependencies`.
func dependenciesSatisfied(inGraph map[*spec.Node]bool, dependencies []string, nodes map[string]*spec.Node) bool {
	for _, dep := range dependencies {
		node := nodes[dep]
		if _, ok := inGraph[node]; !ok {
			return false
		}
	}
	return true
}

// Returns a list of node args that aren't present in the job args map.
func getMissingArgs(n *spec.Node, jobArgs map[string]bool) ([]string, error) {
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
