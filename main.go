package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// sidebarWidth is the fixed width of the resource sidebar.
const sidebarWidth = 244

// object is one dummy Kubernetes object of a kind.
type object struct {
	Name      string
	Namespace string
	Status    string
	Age       string
	// Image and Node are filled for Pods; Restarts counts their restarts.
	Image    string
	Node     string
	Restarts int
}

// resourceKind is one kind in the sidebar with its dummy objects.
type resourceKind struct {
	id, label, group string
	icon             *ui.SVG
	objects          []object
}

// group is one titled section of the sidebar.
type group struct {
	title string
	ids   []string
}

var groups = []group{
	{"Workloads", []string{"pods", "deployments", "statefulsets", "daemonsets", "jobs", "cronjobs"}},
	{"Config", []string{"configmaps", "secrets"}},
	{"Network", []string{"services", "ingresses"}},
	{"Cluster", []string{"nodes", "namespaces", "persistentvolumes"}},
}

// namespaces are the choices of the header's namespace picker, a mock.
var namespaces = []string{"All namespaces", "default", "kube-system", "monitoring", "payments"}

// Icons are inline SVGs tinted by the theme through currentColor.
var (
	podsIcon              = icon(`<path d="M12 2.5 20.5 7.3v9.4L12 21.5 3.5 16.7V7.3z"/><path d="m3.5 7.3 8.5 4.9 8.5-4.9M12 12.2v9.3"/>`)
	deploymentsIcon       = icon(`<path d="m12 2.5 8.5 4.8L12 12.1 3.5 7.3z"/><path d="m3.5 12.1 8.5 4.8 8.5-4.8"/><path d="m3.5 16.7 8.5 4.8 8.5-4.8"/>`)
	statefulsetsIcon      = icon(`<ellipse cx="12" cy="5.5" rx="8" ry="3"/><path d="M4 5.5v13c0 1.7 3.6 3 8 3s8-1.3 8-3v-13"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>`)
	daemonsetsIcon        = icon(`<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>`)
	jobsIcon              = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>`)
	cronjobsIcon          = icon(`<rect x="3" y="5" width="18" height="16" rx="2.5"/><path d="M8 2.8V7M16 2.8V7M3 10h18"/><path d="M12 13.5v3l2 1.2"/>`)
	configmapsIcon        = icon(`<path d="M14 2.8H6.5A2.5 2.5 0 0 0 4 5.3v13.4a2.5 2.5 0 0 0 2.5 2.5h11a2.5 2.5 0 0 0 2.5-2.5V8.3z"/><path d="M14 2.8v5.5h5.5M8 13h8M8 17h5.5"/>`)
	secretsIcon           = icon(`<rect x="4" y="10" width="16" height="11" rx="2.5"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/><path d="M12 14.5v2.5"/>`)
	servicesIcon          = icon(`<circle cx="12" cy="4.8" r="2.3"/><circle cx="5" cy="19" r="2.3"/><circle cx="19" cy="19" r="2.3"/><path d="M12 7.1v4.4M6.6 17.2 11 12.5M17.4 17.2 13 12.5"/>`)
	ingressesIcon         = icon(`<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z"/>`)
	nodesIcon             = icon(`<rect x="4" y="4" width="16" height="16" rx="2.5"/><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2"/>`)
	namespacesIcon        = icon(`<path d="M3 6.5A2.5 2.5 0 0 1 5.5 4h3.2l2 2.5h7.8A2.5 2.5 0 0 1 21 9v8.5a2.5 2.5 0 0 1-2.5 2.5h-13A2.5 2.5 0 0 1 3 17.5z"/>`)
	persistentvolumesIcon = icon(`<rect x="3" y="4" width="18" height="7" rx="2"/><path d="M7 7.5h.01"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 16.5h.01M16 7.5h1M16 16.5h1"/>`)
)

func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

// Resources are the mock kinds of the sidebar, with their dummy objects.
var resources = []*resourceKind{
	{"pods", "Pods", "Workloads", podsIcon, []object{
		{"api-7d9f8c-x2k4p", "default", "Running", "18m", "ghcr.io/acme/api:2.14.0", "node-1", 0},
		{"web-5c6d7f-m9q1z", "default", "Running", "2h", "ghcr.io/acme/web:1.9.3", "node-2", 0},
		{"worker-6b8c9d-t4w7r", "default", "Running", "1d", "ghcr.io/acme/worker:0.8.1", "node-3", 2},
		{"cache-1d2e3f-k7l9m", "default", "Pending", "3m", "redis:7.4", "—", 0},
		{"payments-9e8d7c-p8s6v", "payments", "CrashLoopBackOff", "42m", "ghcr.io/acme/payments:3.0.0", "node-2", 7},
		{"sync-2f4d1a-h3j5n", "kube-system", "Completed", "9m", "busybox:1.36", "node-1", 0},
	}},
	{"deployments", "Deployments", "Workloads", deploymentsIcon, []object{
		{"api-gateway", "default", "3/3 ready", "4d", "", "", 0},
		{"web-frontend", "default", "2/2 ready", "4d", "", "", 0},
		{"worker-pool", "default", "1/1 ready", "6d", "", "", 0},
		{"payments-api", "payments", "Progressing", "42m", "", "", 0},
	}},
	{"statefulsets", "StatefulSets", "Workloads", statefulsetsIcon, []object{
		{"postgres", "default", "1/1 ready", "12d", "", "", 0},
		{"kafka", "monitoring", "3/3 ready", "12d", "", "", 0},
	}},
	{"daemonsets", "DaemonSets", "Workloads", daemonsetsIcon, []object{
		{"node-agent", "kube-system", "3/3 ready", "12d", "", "", 0},
		{"log-shipper", "monitoring", "3/3 ready", "10d", "", "", 0},
	}},
	{"jobs", "Jobs", "Workloads", jobsIcon, []object{
		{"migrate-2891", "default", "Complete", "2h", "", "", 0},
		{"backup-daily-28912", "kube-system", "Complete", "5h", "", "", 0},
		{"reindex-42", "default", "Failed", "1d", "", "", 0},
	}},
	{"cronjobs", "CronJobs", "Workloads", cronjobsIcon, []object{
		{"backup-daily", "kube-system", "Active", "12d", "", "", 0},
		{"report-hourly", "default", "Suspended", "12d", "", "", 0},
		{"cleanup-weekly", "default", "Active", "8d", "", "", 0},
	}},
	{"configmaps", "ConfigMaps", "Config", configmapsIcon, []object{
		{"app-config", "default", "6 keys", "12d", "", "", 0},
		{"feature-flags", "default", "12 keys", "12d", "", "", 0},
		{"nginx-conf", "default", "1 key", "12d", "", "", 0},
		{"cluster-info", "kube-system", "2 keys", "12d", "", "", 0},
		{"grafana-datasources", "monitoring", "3 keys", "10d", "", "", 0},
	}},
	{"secrets", "Secrets", "Config", secretsIcon, []object{
		{"regcred", "default", "1 key", "12d", "", "", 0},
		{"db-credentials", "default", "2 keys", "12d", "", "", 0},
		{"tls-acme", "default", "2 keys", "30d", "", "", 0},
		{"api-token", "payments", "1 key", "8d", "", "", 0},
		{"grafana-admin", "monitoring", "2 keys", "10d", "", "", 0},
	}},
	{"services", "Services", "Network", servicesIcon, []object{
		{"kubernetes", "default", "ClusterIP", "12d", "", "", 0},
		{"api", "default", "ClusterIP", "12d", "", "", 0},
		{"web", "default", "LoadBalancer", "12d", "", "", 0},
		{"postgres", "default", "ClusterIP", "12d", "", "", 0},
		{"grafana", "monitoring", "NodePort", "10d", "", "", 0},
	}},
	{"ingresses", "Ingresses", "Network", ingressesIcon, []object{
		{"api-ingress", "default", "Ready", "12d", "", "", 0},
		{"web-ingress", "default", "Ready", "12d", "", "", 0},
		{"grafana-ingress", "monitoring", "Ready", "10d", "", "", 0},
	}},
	{"nodes", "Nodes", "Cluster", nodesIcon, []object{
		{"node-1", "—", "Ready", "12d", "", "", 0},
		{"node-2", "—", "Ready", "12d", "", "", 0},
		{"node-3", "—", "NotReady", "3h", "", "", 0},
	}},
	{"namespaces", "Namespaces", "Cluster", namespacesIcon, []object{
		{"default", "—", "Active", "12d", "", "", 0},
		{"kube-system", "—", "Active", "12d", "", "", 0},
		{"kube-public", "—", "Active", "12d", "", "", 0},
		{"monitoring", "—", "Active", "10d", "", "", 0},
		{"payments", "—", "Terminating", "42m", "", "", 0},
	}},
	{"persistentvolumes", "PersistentVolumes", "Cluster", persistentvolumesIcon, []object{
		{"pv-postgres-0", "—", "Bound", "12d", "", "", 0},
		{"pv-kafka-0", "—", "Bound", "12d", "", "", 0},
		{"pv-backup", "—", "Available", "5h", "", "", 0},
		{"pv-media-0", "—", "Released", "1d", "", "", 0},
	}},
}

var resourcesByID = func() map[string]*resourceKind {
	m := make(map[string]*resourceKind, len(resources))
	for _, k := range resources {
		m[k.id] = k
	}
	return m
}()

func resource(id string) *resourceKind {
	if k := resourcesByID[id]; k != nil {
		return k
	}
	return resources[0]
}

// app is the state of the window's view.
type app struct {
	kind     string
	lastKind string
	filter   string

	namespace string
	row       int
	table     ui.ListState

	workloadsOpen bool
	configOpen    bool
	networkOpen   bool
	clusterOpen   bool
}

func newApp() *app {
	return &app{
		kind:          "pods",
		lastKind:      "pods",
		namespace:     "All namespaces",
		workloadsOpen: true,
		configOpen:    true,
		networkOpen:   true,
		clusterOpen:   true,
	}
}

func (a *app) groupOpen(group string) *bool {
	switch group {
	case "Workloads":
		return &a.workloadsOpen
	case "Config":
		return &a.configOpen
	case "Network":
		return &a.networkOpen
	default:
		return &a.clusterOpen
	}
}

// view builds the window's interface from the state, for every frame.
func (a *app) view(c *ui.Context) {
	// A new kind starts at its first object with a fresh table state, so a
	// stale row or scroll offset never points past the new object slice.
	if a.kind != a.lastKind {
		a.lastKind = a.kind
		a.table = ui.ListState{}
		a.row = 0
	}
	a.shell(c)
}

func (a *app) shell(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Background(t.Background).Children(func() {
		a.sidebar(c)
		ui.Divider(c)
		a.pane(c)
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(sidebarWidth).FillHeight().Background(t.Surface).Children(func() {
		// Cluster context, a mock.
		ui.Row(c).Padding(14, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(999).Background(t.Success)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "docker-desktop").FontSize(12).Bold().SingleLine()
				ui.Text(c, "Kubernetes v1.31.2").FontSize(10).TextColor(t.TextMuted).SingleLine()
			})
			ui.Text(c, "⌄").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Row(c).Padding(4, 10).Children(func() {
			ui.TextInput(c, &a.filter).Placeholder("Filter resources").Grow(1)
		})
		a.kindList(c)
	})
}

// kindList builds the sidebar's sections and items, filtered by a.filter.
func (a *app) kindList(c *ui.Context) {
	found := false
	ui.Sidebar(c, &a.kind, func() {
		for _, g := range groups {
			items := a.groupItems(g)
			if len(items) == 0 {
				continue
			}
			found = true
			ui.SidebarSection(c, g.title, a.groupOpen(g.title), func() {
				for _, k := range items {
					ui.SidebarItem(c, k.id, k.icon, k.label).Children(func() {
						ui.Badge(c, fmt.Sprint(len(k.objects)))
					})
				}
			})
		}
		if !found {
			ui.Text(c, "No resources").FontSize(11).TextColor(c.Theme().TextMuted).Padding(8)
		}
	}).Grow(1)
}

// groupItems returns the kinds of a group matching the filter.
func (a *app) groupItems(g group) []*resourceKind {
	query := strings.ToLower(a.filter)
	var out []*resourceKind
	for _, id := range g.ids {
		k := resource(id)
		if query == "" || strings.Contains(strings.ToLower(k.label), query) {
			out = append(out, k)
		}
	}
	return out
}

func (a *app) pane(c *ui.Context) {
	k := resource(a.kind)
	ui.Column(c).Grow(1).Children(func() {
		a.header(c, k)
		ui.Divider(c)
		ui.Row(c).Grow(1).Children(func() {
			a.objectTable(c, k)
			ui.Divider(c)
			a.inspector(c, k)
		})
	})
}

func (a *app) header(c *ui.Context, k *resourceKind) {
	t := c.Theme()
	ui.Row(c).Padding(16, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(36, 36).Radius(8).Background(t.Accent).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, k.icon).FontSize(20).TextColor(t.AccentText)
		})
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, k.label).FontSize(20).Bold().SingleLine()
			ui.Text(c, fmt.Sprintf("%d objects · %s", len(k.objects), a.namespace)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		ui.Select(c, &a.namespace, namespaces).Width(160)
		ui.Button(c, "Refresh")
		ui.PrimaryButton(c, "Create")
	})
}

// columns returns the table's columns: Pods show their restart count.
func (a *app) columns(k *resourceKind) []ui.TableColumn {
	cols := []ui.TableColumn{
		{Title: "Name"},
		{Title: "Namespace", Width: 110},
		{Title: "Status", Width: 150},
	}
	if k.id == "pods" {
		cols = append(cols, ui.TableColumn{Title: "Restarts", Width: 76, Align: ui.End})
	}
	return append(cols, ui.TableColumn{Title: "Age", Width: 64, Align: ui.End})
}

func (a *app) objectTable(c *ui.Context, k *resourceKind) {
	t := c.Theme()
	a.table.Selected = &a.row
	a.table.Key = func(i int) any { return k.objects[i].Name }
	a.table.Label = func(i int) string { return k.objects[i].Name }
	cols := a.columns(k)
	ui.Table(c, &a.table, cols, len(k.objects), func(row, col int) {
		o := k.objects[row]
		// A chosen row fills with the accent, so its secondary text and its
		// status dot follow the accent's text color.
		chosen := row == a.row
		muted, dot := t.TextMuted, statusColor(t, o.Status)
		if chosen {
			muted, dot = t.AccentText, t.AccentText
		}
		switch {
		case col == 0:
			ui.Text(c, o.Name).SingleLine()
		case col == 1:
			ui.Text(c, o.Namespace).TextColor(muted).SingleLine()
		case col == 2:
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(7, 7).Radius(999).Background(dot)
				ui.Text(c, o.Status).SingleLine()
			})
		case col == 3 && k.id == "pods":
			ui.Text(c, fmt.Sprint(o.Restarts)).TextColor(muted).SingleLine()
		case col == len(cols)-1:
			ui.Text(c, o.Age).TextColor(muted).SingleLine()
		}
	}).Grow(1)
}

func (a *app) inspector(c *ui.Context, k *resourceKind) {
	t := c.Theme()
	o := k.objects[clamp(a.row, 0, len(k.objects)-1)]
	ui.Scroll(c).Width(320).FillHeight().Children(func() {
		ui.Column(c).Padding(16, 18).Gap(14).Children(func() {
			ui.Text(c, strings.ToUpper(k.label)).FontSize(10).FontWeight(600).TextColor(t.TextMuted).SingleLine()
			ui.Text(c, o.Name).FontSize(16).Bold()
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(7, 7).Radius(999).Background(statusColor(t, o.Status))
				ui.Text(c, o.Status).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			ui.Divider(c)
			sectionTitle(c, "CURRENT")
			detailRow(c, "Namespace", o.Namespace)
			detailRow(c, "Age", o.Age)
			if o.Image != "" {
				detailRow(c, "Image", o.Image)
				detailRow(c, "Node", o.Node)
				detailRow(c, "Restarts", fmt.Sprint(o.Restarts))
			}
			ui.Divider(c)
			sectionTitle(c, "LABELS")
			ui.Row(c).Gap(5).Wrap().Children(func() {
				for _, label := range objectLabels(o) {
					ui.Badge(c, label)
				}
			})
			ui.Divider(c)
			sectionTitle(c, "EVENTS")
			for _, e := range objectEvents(o) {
				eventRow(c, e)
			}
		})
	})
}

type event struct {
	kind, reason, message, age string
}

func objectEvents(o object) []event {
	switch o.Status {
	case "Failed", "CrashLoopBackOff":
		return []event{
			{"Warning", "BackOff", "Back-off restarting failed container", "2m"},
			{"Warning", "Failed", "Container exited with code 1", "3m"},
		}
	case "Pending":
		return []event{
			{"Normal", "Scheduled", "Successfully assigned to node-1", "3m"},
			{"Warning", "FailedScheduling", "0/3 nodes are available", "3m"},
		}
	case "NotReady", "Terminating":
		return []event{
			{"Warning", "NodeNotReady", "Kubelet stopped posting node status", "4m"},
		}
	default:
		node := o.Node
		if node == "" || node == "—" {
			node = "node-1"
		}
		return []event{
			{"Normal", "Scheduled", "Assigned to " + node, "18m"},
			{"Normal", "Pulled", "Container image already present", "17m"},
		}
	}
}

func objectLabels(o object) []string {
	base := o.Name
	if i := strings.IndexByte(base, '-'); i > 0 {
		base = base[:i]
	}
	return []string{"app=" + base, "env=production", "managed-by=mygo"}
}

func sectionTitle(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(10).FontWeight(600).TextColor(c.Theme().TextMuted).SingleLine()
}

func detailRow(c *ui.Context, key, value string) {
	t := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
		ui.Text(c, key).FontSize(11).TextColor(t.TextMuted).Width(76).SingleLine()
		ui.Text(c, value).FontSize(11).Grow(1)
	})
}

func eventRow(c *ui.Context, e event) {
	t := c.Theme()
	color := t.Success
	label := e.kind
	if e.kind == "Warning" {
		color = t.Warning
		label = "Warning"
	}
	ui.Column(c).Gap(3).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(999).Background(color)
			ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(color).SingleLine()
			ui.Text(c, e.age).FontSize(10).TextColor(t.TextMuted).Grow(1).TextAlign(ui.End).SingleLine()
		})
		ui.Text(c, e.reason+" · "+e.message).FontSize(11)
	})
}

func statusColor(t *ui.Theme, status string) ui.Color {
	s := strings.ToLower(status)
	switch {
	case strings.Contains(s, "notready"), strings.Contains(s, "failed"),
		strings.Contains(s, "crashloop"), strings.Contains(s, "error"):
		return t.Danger
	case strings.Contains(s, "terminating"), strings.Contains(s, "pending"),
		strings.Contains(s, "progressing"), strings.Contains(s, "suspended"),
		strings.Contains(s, "released"):
		return t.Warning
	case strings.Contains(s, "ready"), strings.Contains(s, "running"),
		strings.Contains(s, "active"), strings.Contains(s, "bound"),
		strings.Contains(s, "complete"), strings.Contains(s, "available"),
		s == "clusterip", s == "loadbalancer", s == "nodeport":
		return t.Success
	default:
		return t.TextMuted
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func main() {
	a := newApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "Kubernetes",
			Width:     1180,
			Height:    720,
			MinWidth:  960,
			MinHeight: 600,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
