package dag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"google.golang.org/protobuf/encoding/protojson"
)

func pprintDag(dag *primitive.Dag, full bool) {
	if dag == nil {
		fmt.Println("<nil dag>")
		return
	}

	printDagGraph(dag, full, 0)

	if !full {
		return
	}

	fmt.Println()
	fmt.Println("Full DAG:")
	out, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(dag)
	if err != nil {
		fmt.Printf("<marshal dag: %v>\n", err)
		return
	}
	fmt.Println(string(out))
}

func printDagGraph(dag *primitive.Dag, full bool, depth int) {
	indent := strings.Repeat("  ", depth)
	if dag == nil {
		fmt.Printf("%s<nil dag>\n", indent)
		return
	}

	fmt.Printf("%sDAG %s\n", indent, dagTitle(dag))
	if full {
		printDagDetails(dag, indent+"  ")
	}

	nodes := dag.GetNodes()
	if len(nodes) == 0 {
		fmt.Printf("%s  <empty>\n", indent)
		return
	}

	nodeByID, outgoing, incoming := indexDag(dag)
	seen := make(map[string]bool, len(nodes))
	roots := rootNodeIDs(nodes, incoming)

	for i, id := range roots {
		renderNodeGraph(nodeByID[id], outgoing, nodeByID, seen, indent+"  ", i == len(roots)-1, full)
	}

	printDetachedNodes(nodes, seen, outgoing, nodeByID, indent+"  ", full)
}

func renderNodeGraph(
	node *primitive.Dag_Node,
	outgoing map[string][]*primitive.Dag_Edge,
	nodeByID map[string]*primitive.Dag_Node,
	seen map[string]bool,
	prefix string,
	last bool,
	full bool,
) {
	branch, childPrefix := "+-- ", "|   "
	if last {
		branch, childPrefix = "`-- ", "    "
	}

	if node == nil {
		fmt.Printf("%s%s<missing node>\n", prefix, branch)
		return
	}

	id := node.GetId()
	alreadySeen := seen[id]
	fmt.Printf("%s%s%s", prefix, branch, nodeBox(node))
	if alreadySeen {
		fmt.Print(" (already shown)")
	}
	fmt.Println()

	if full {
		printNodeDetails(node, prefix+childPrefix)
	}

	if alreadySeen {
		return
	}
	seen[id] = true

	if sub := node.GetSubDag(); sub != nil {
		printDagGraph(sub, full, strings.Count(prefix+childPrefix, "  "))
	}

	edges := outgoing[id]
	for i, edge := range edges {
		renderEdgeGraph(edge, outgoing, nodeByID, seen, prefix+childPrefix, i == len(edges)-1, full)
	}
}

func renderEdgeGraph(
	edge *primitive.Dag_Edge,
	outgoing map[string][]*primitive.Dag_Edge,
	nodeByID map[string]*primitive.Dag_Node,
	seen map[string]bool,
	prefix string,
	last bool,
	full bool,
) {
	branch, childPrefix := "+-- ", "|   "
	if last {
		branch, childPrefix = "`-- ", "    "
	}
	if edge == nil {
		fmt.Printf("%s%s<nil edge>\n", prefix, branch)
		return
	}

	target := nodeByID[edge.GetTarget()]
	if target == nil {
		fmt.Printf("%s%s(%s)--> <missing:%s>\n", prefix, branch, edgeCondition(edge), edge.GetTarget())
		return
	}

	fmt.Printf("%s%s(%s)-->\n", prefix, branch, edgeCondition(edge))
	renderNodeGraph(target, outgoing, nodeByID, seen, prefix+childPrefix, true, full)
}

func printDetachedNodes(
	nodes []*primitive.Dag_Node,
	seen map[string]bool,
	outgoing map[string][]*primitive.Dag_Edge,
	nodeByID map[string]*primitive.Dag_Node,
	prefix string,
	full bool,
) {
	var detached []*primitive.Dag_Node
	for _, node := range nodes {
		if node != nil && !seen[node.GetId()] {
			detached = append(detached, node)
		}
	}
	if len(detached) == 0 {
		return
	}

	fmt.Printf("%sDetached or cyclic nodes:\n", prefix)
	for i, node := range detached {
		renderNodeGraph(node, outgoing, nodeByID, seen, prefix+"  ", i == len(detached)-1, full)
	}
}

func indexDag(dag *primitive.Dag) (map[string]*primitive.Dag_Node, map[string][]*primitive.Dag_Edge, map[string]int) {
	nodeByID := make(map[string]*primitive.Dag_Node, len(dag.GetNodes()))
	for _, node := range dag.GetNodes() {
		if node == nil {
			continue
		}
		nodeByID[node.GetId()] = node
	}

	outgoing := make(map[string][]*primitive.Dag_Edge)
	incoming := make(map[string]int)
	for _, edge := range dag.GetEdges() {
		if edge == nil {
			continue
		}
		outgoing[edge.GetSource()] = append(outgoing[edge.GetSource()], edge)
		incoming[edge.GetTarget()]++
	}
	for _, edges := range outgoing {
		sort.SliceStable(edges, func(i, j int) bool {
			return edges[i].GetTarget() < edges[j].GetTarget()
		})
	}

	return nodeByID, outgoing, incoming
}

func rootNodeIDs(nodes []*primitive.Dag_Node, incoming map[string]int) []string {
	roots := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if incoming[node.GetId()] == 0 {
			roots = append(roots, node.GetId())
		}
	}
	if len(roots) == 0 {
		for _, node := range nodes {
			if node != nil {
				roots = append(roots, node.GetId())
			}
		}
	}
	return roots
}

func dagTitle(dag *primitive.Dag) string {
	parts := []string{fmt.Sprintf("id=%q", dag.GetId()), fmt.Sprintf("status=%s", dag.GetStatus())}
	parts = append(parts, fmt.Sprintf("nodes=%d", len(dag.GetNodes())), fmt.Sprintf("edges=%d", len(dag.GetEdges())))
	if dag.GetScheduling().GetIsSubDag() {
		parts = append(parts, "sub_dag=true")
	}
	return strings.Join(parts, " ")
}

func nodeBox(node *primitive.Dag_Node) string {
	if node == nil {
		return "[<nil>]"
	}

	kind := "node"
	switch node.GetVariant().(type) {
	case *primitive.Dag_Node_TaskState_:
		kind = "task"
	case *primitive.Dag_Node_SubDag:
		kind = "sub_dag"
	case *primitive.Dag_Node_DagRef_:
		kind = "dag_ref"
	}

	return fmt.Sprintf("[%s | %s | %s]", node.GetId(), kind, node.GetStatus())
}

func printDagDetails(dag *primitive.Dag, indent string) {
	if scheduling := dag.GetScheduling(); scheduling != nil {
		fmt.Printf("%sscheduling: max_parallelism=%d on_node_failure=%s is_sub_dag=%t\n",
			indent,
			scheduling.GetMaxParallelism(),
			scheduling.GetOnNodeFailure(),
			scheduling.GetIsSubDag(),
		)
	}
	if len(dag.GetMetadata()) > 0 {
		fmt.Printf("%smetadata: %s\n", indent, formatStringMap(dag.GetMetadata()))
	}
	if execution := dag.GetExecution(); execution != nil {
		fmt.Printf("%sexecution: status=%s failed_node=%q failures=%d\n",
			indent,
			execution.GetStatus(),
			execution.GetFailedNodeId(),
			len(execution.GetFailures()),
		)
	}
	if input := dag.GetInput(); input != nil {
		fmt.Printf("%sinput: %s\n", indent, input.GetTypeUrl())
	}
}

func printNodeDetails(node *primitive.Dag_Node, indent string) {
	if node.GetExecutionId() != "" {
		fmt.Printf("%sexecution_id: %s\n", indent, node.GetExecutionId())
	}
	if scheduling := node.GetScheduling(); scheduling != nil {
		fmt.Printf("%sscheduling: always_run=%t join=%s priority=%d retry_policy=%t\n",
			indent,
			scheduling.GetAlwaysRun(),
			scheduling.GetJoinPolicy(),
			scheduling.GetPriority(),
			scheduling.GetRetryPolicy() != nil,
		)
	}
	if len(node.GetMetadata()) > 0 {
		fmt.Printf("%smetadata: %s\n", indent, formatStringMap(node.GetMetadata()))
	}

	switch variant := node.GetVariant().(type) {
	case *primitive.Dag_Node_TaskState_:
		task := variant.TaskState
		fmt.Printf("%stask: handler=%q locus=%s\n", indent, task.GetHandlerName(), task.GetLocus())
		if task.GetInput() != nil {
			fmt.Printf("%sinput: %s\n", indent, task.GetInput().GetTypeUrl())
		}
		if task.GetOutput() != nil {
			fmt.Printf("%soutput: %s\n", indent, task.GetOutput().GetTypeUrl())
		}
	case *primitive.Dag_Node_SubDag:
		if variant.SubDag == nil {
			fmt.Printf("%ssub_dag: <nil>\n", indent)
		}
	case *primitive.Dag_Node_DagRef_:
		fmt.Printf("%sdag_ref: %q\n", indent, variant.DagRef.GetDagId())
	default:
		fmt.Printf("%svariant: <none>\n", indent)
	}

	if execution := node.GetExecution(); execution != nil {
		fmt.Printf("%sexecution: status=%s failures=%d\n", indent, execution.GetStatus(), len(execution.GetFailures()))
		if retry := execution.GetRetryState(); retry != nil {
			fmt.Printf("%sretry: attempt=%d last_error=%q\n", indent, retry.GetAttempt(), retry.GetLastError())
		}
		if failure := execution.GetFailure(); failure != nil {
			fmt.Printf("%sfailure: code=%q source=%q phase=%q message=%q\n",
				indent,
				failure.GetCode(),
				failure.GetSource(),
				failure.GetPhase(),
				failure.GetMessage(),
			)
		}
	}
}

func edgeCondition(edge *primitive.Dag_Edge) string {
	if edge == nil {
		return "<nil edge>"
	}
	switch condition := edge.GetCondition().(type) {
	case *primitive.Dag_Edge_PredicateName:
		return "predicate:" + condition.PredicateName
	case *primitive.Dag_Edge_OnStatus:
		return "status:" + condition.OnStatus.String()
	default:
		return "completed"
	}
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", key, values[key]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
