package printer

import (
	"fmt"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heptio/developer-dash/internal/cache"
	"github.com/heptio/developer-dash/internal/view/component"
	"github.com/heptio/developer-dash/internal/view/flexlayout"
)

// ReplicaSetListHandler is a printFunc that lists deployments
func ReplicaSetListHandler(list *appsv1.ReplicaSetList, opts Options) (component.ViewComponent, error) {
	if list == nil {
		return nil, errors.New("nil list")
	}

	cols := component.NewTableCols("Name", "Labels", "Status", "Age", "Containers", "Selector")
	tbl := component.NewTable("ReplicaSets", cols)

	for _, rs := range list.Items {
		row := component.TableRow{}
		replicasetPath := gvkPath(rs.TypeMeta.APIVersion, rs.TypeMeta.Kind, rs.Name)
		row["Name"] = component.NewLink("", rs.Name, replicasetPath)
		row["Labels"] = component.NewLabels(rs.Labels)

		status := fmt.Sprintf("%d/%d", rs.Status.AvailableReplicas, rs.Status.Replicas)
		row["Status"] = component.NewText(status)

		ts := rs.CreationTimestamp.Time
		row["Age"] = component.NewTimestamp(ts)

		containers := component.NewContainers()
		for _, c := range rs.Spec.Template.Spec.Containers {
			containers.Add(c.Name, c.Image)
		}
		row["Containers"] = containers
		row["Selector"] = printSelector(rs.Spec.Selector)

		tbl.Add(row)
	}
	return tbl, nil
}

// ReplicaSetHandler is a printFunc that prints a ReplicaSets.
func ReplicaSetHandler(rs *appsv1.ReplicaSet, options Options) (component.ViewComponent, error) {
	fl := flexlayout.New()

	configSection := fl.AddSection()

	rsConfigGen := NewReplicaSetConfiguration(rs)
	configView, err := rsConfigGen.Create()
	if err != nil {
		return nil, err
	}

	if err := configSection.Add(configView, 16); err != nil {
		return nil, errors.Wrap(err, "add replicaset config to layout")
	}

	summarySection := fl.AddSection()

	rsSummaryGen := NewReplicaSetStatus(rs)
	statusView, err := rsSummaryGen.Create(options.Cache)
	if err != nil {
		return nil, err
	}

	if err := summarySection.Add(statusView, 8); err != nil {
		return nil, errors.Wrap(err, "add replicaset summary to layout")
	}

	view := fl.ToComponent("Summary")

	return view, nil
}

// ReplicaSetConfiguration generates a replicaset configuration
type ReplicaSetConfiguration struct {
	replicaset *appsv1.ReplicaSet
}

// NewReplicaSetConfiguration creates an instance of ReplicaSetConfiguration
func NewReplicaSetConfiguration(rs *appsv1.ReplicaSet) *ReplicaSetConfiguration {
	return &ReplicaSetConfiguration{
		replicaset: rs,
	}
}

// Create generates a replicaset configuration summary
func (rc *ReplicaSetConfiguration) Create() (*component.Summary, error) {
	if rc == nil || rc.replicaset == nil {
		return nil, errors.New("replicaset is nil")
	}

	rs := rc.replicaset

	sections := component.SummarySections{}

	if controllerRef := metav1.GetControllerOf(rs); controllerRef != nil {
		sections = append(sections, component.SummarySection{
			Header:  "Controlled By",
			Content: linkForOwner(controllerRef),
		})
	}

	current := fmt.Sprintf("%d", rs.Status.ReadyReplicas)

	if desired := rs.Spec.Replicas; desired != nil {
		desiredReplicas := fmt.Sprintf("%d", *desired)
		status := fmt.Sprintf("Current %s / Desired %s", current, desiredReplicas)
		sections.AddText("Replica Status", status)
	}

	replicas := fmt.Sprintf("%d", rs.Status.Replicas)
	sections.AddText("Replicas", replicas)

	summary := component.NewSummary("Configuration", sections...)

	return summary, nil
}

// ReplicaSetStatus generates a replicaset status
type ReplicaSetStatus struct {
	replicaset *appsv1.ReplicaSet
}

// NewReplicaSetStatus creates an instance of ReplicaSetStatus
func NewReplicaSetStatus(rs *appsv1.ReplicaSet) *ReplicaSetStatus {
	return &ReplicaSetStatus{
		replicaset: rs,
	}
}

// Create generates a replicaset status quadrant
func (rs *ReplicaSetStatus) Create(c cache.Cache) (*component.Quadrant, error) {
	if rs == nil || rs.replicaset == nil {
		return nil, errors.New("replicaset is nil")
	}
	pods, err := listPods(rs.replicaset.Namespace, rs.replicaset.Spec.Selector, rs.replicaset.GetUID(), c)
	if err != nil {
		return nil, err
	}

	ps := createPodStatus(pods)

	quadrant := component.NewQuadrant()
	if err := quadrant.Set(component.QuadNW, "Running", fmt.Sprintf("%d", ps.Running)); err != nil {
		return nil, errors.New("unable to set quadrant nw")
	}
	if err := quadrant.Set(component.QuadNE, "Waiting", fmt.Sprintf("%d", ps.Waiting)); err != nil {
		return nil, errors.New("unable to set quadrant ne")
	}
	if err := quadrant.Set(component.QuadSW, "Succeeded", fmt.Sprintf("%d", ps.Succeeded)); err != nil {
		return nil, errors.New("unable to set quadrant sw")
	}
	if err := quadrant.Set(component.QuadSE, "Failed", fmt.Sprintf("%d", ps.Failed)); err != nil {
		return nil, errors.New("unable to set quadrant se")
	}

	return quadrant, nil
}
