package ui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"k8s.io/client-go/kubernetes"
	"ktwins/internal/data"
	"ktwins/internal/theme"
)

// Dashboard encapsula estado e handlers da UI.
type Dashboard struct {
	ns        string
	clientset *kubernetes.Clientset

	app *tview.Application

	modalLogs      *tview.TextView
	infoPopup      *tview.TextView
	overview       *tview.TextView
	alertsView     *tview.TextView
	eventsView     *tview.TextView
	namespacesView *tview.TextView
	infraView      *tview.TextView
	configView     *tview.TextView
	storageView    *tview.TextView
	networkView    *tview.TextView
	workloadsView  *tview.TextView
	podsView       *tview.TextView
	metricsView    *tview.TextView

	workloadsPage *tview.Flex
	clusterPage   *tview.Flex
	networkPage   *tview.Flex
	metricsPage   *tview.Flex
	pages         *tview.Pages
	pageIndicator *tview.TextView
	header        *tview.Flex
	root          *tview.Flex

	currentPage string
	pageOrder   []string

	contentCache   map[*tview.TextView]string
	browseBox      *tview.TextView
	selectedLine   int
	modalOpen      bool
	restoreFocus   tview.Primitive
	nsList         []string
	updateMu       sync.Mutex
	borderDefaults map[*tview.TextView]tcell.Color
	originalTitles map[*tview.TextView]string
	updateCh       chan struct{}
	ticker         *time.Ticker
}

func NewDashboard(ns string, clientset *kubernetes.Clientset) *Dashboard {
	d := &Dashboard{
		ns:             ns,
		clientset:      clientset,
		app:            tview.NewApplication(),
		modalLogs:      newTextArea("LOGS"),
		infoPopup:      newTextArea("INFO"),
		overview:       newBox("OVERVIEW"),
		alertsView:     newBox("ALERTS"),
		eventsView:     newBox("EVENTS"),
		namespacesView: newBox("NAMESPACES"),
		infraView:      newBox("INFRA"),
		configView:     newBox("CONFIG"),
		storageView:    newBox("STORAGE"),
		networkView:    newBox("NETWORK"),
		workloadsView:  newBox("WORKLOADS"),
		podsView:       newBox("PODS"),
		metricsView:    newBox("POD METRICS"),
		pageIndicator:  tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		contentCache:   map[*tview.TextView]string{},
		originalTitles: map[*tview.TextView]string{},
		updateCh:       make(chan struct{}, 1),
		currentPage:    "workloads",
		pageOrder:      []string{"workloads", "network", "cluster", "metrics"},
		selectedLine:   0,
	}

	d.modalLogs.SetTitle("LOGS")
	d.infoPopup.SetTitle("INFO")

	alertEventColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.alertsView, 0, 1, false).
		AddItem(d.eventsView, 0, 1, false)

	header := tview.NewFlex().
		AddItem(d.namespacesView, 0, 1, false).
		AddItem(d.overview, 0, 2, false).
		AddItem(alertEventColumn, 0, 1, false)
	d.header = header

	d.workloadsPage = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.workloadsView, 0, 1, true).
		AddItem(d.podsView, 0, 1, false)

	d.clusterPage = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.infraView, 0, 1, false).
		AddItem(d.configView, 0, 1, false).
		AddItem(d.storageView, 0, 1, false)

	d.networkPage = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.networkView, 0, 1, false)

	d.metricsPage = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.metricsView, 0, 1, false)

	d.pages = tview.NewPages().
		AddPage("workloads", d.workloadsPage, true, true).
		AddPage("network", d.networkPage, true, false).
		AddPage("cluster", d.clusterPage, true, false).
		AddPage("metrics", d.metricsPage, true, false)

	d.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 9, 0, false).
		AddItem(d.pages, 0, 1, true).
		AddItem(d.pageIndicator, 1, 0, false)

	d.setPlaceholders()
	d.borderDefaults = map[*tview.TextView]tcell.Color{
		d.namespacesView: tcell.ColorWhite,
		d.overview:       tcell.ColorLightSkyBlue,
		d.alertsView:     tcell.ColorGreen,
		d.eventsView:     tcell.ColorWheat,
		d.infraView:      tcell.ColorPurple,
		d.configView:     tcell.ColorPurple,
		d.networkView:    tcell.ColorPurple,
		d.storageView:    tcell.ColorPurple,
		d.workloadsView:  tcell.ColorPurple,
		d.podsView:       tcell.ColorPurple,
		d.metricsView:    tcell.ColorPurple,
	}

	return d
}

func (d *Dashboard) setPlaceholders() {
	d.overview.SetText("Carregando...")
	d.alertsView.SetText("Carregando...")
	d.eventsView.SetText("Carregando...")
	d.workloadsView.SetText("Carregando...")
	d.podsView.SetText("Carregando...")
	d.infraView.SetText("Carregando...")
	d.configView.SetText("Carregando...")
	d.networkView.SetText("Carregando...")
	d.storageView.SetText("Carregando...")
	d.metricsView.SetText("Carregando...")
}

func (d *Dashboard) allBoxes() []*tview.TextView {
	return []*tview.TextView{d.workloadsView, d.podsView, d.infraView, d.configView, d.storageView, d.networkView, d.metricsView}
}

func (d *Dashboard) focusOrder() []*tview.TextView {
	base := []*tview.TextView{}
	switch d.currentPage {
	case "workloads":
		if strings.TrimSpace(d.contentCache[d.workloadsView]) != "" {
			base = append(base, d.workloadsView)
		}
		if strings.TrimSpace(d.contentCache[d.podsView]) != "" {
			base = append(base, d.podsView)
		}
	case "network":
		if strings.TrimSpace(d.contentCache[d.networkView]) != "" {
			base = append(base, d.networkView)
		}
	case "cluster":
		if strings.TrimSpace(d.contentCache[d.infraView]) != "" {
			base = append(base, d.infraView)
		}
		if strings.TrimSpace(d.contentCache[d.configView]) != "" {
			base = append(base, d.configView)
		}
		if strings.TrimSpace(d.contentCache[d.storageView]) != "" {
			base = append(base, d.storageView)
		}
	case "metrics":
		if strings.TrimSpace(d.contentCache[d.metricsView]) != "" {
			base = append(base, d.metricsView)
		}
	}
	return base
}

func (d *Dashboard) highlightFocus() {
	focused := d.app.GetFocus()
	for _, b := range d.allBoxes() {
		if b == focused {
			b.SetBorderColor(tcell.ColorOrange)
		} else {
			if def, ok := d.borderDefaults[b]; ok {
				b.SetBorderColor(def)
			} else {
				b.SetBorderColor(tcell.ColorWhite)
			}
		}
	}
	d.overview.SetBorderColor(d.borderDefaults[d.overview])
	d.alertsView.SetBorderColor(d.borderDefaults[d.alertsView])
	d.eventsView.SetBorderColor(d.borderDefaults[d.eventsView])
	if d.browseBox != nil {
		d.browseBox.SetBorderColor(tcell.ColorGreen)
	}
}

func (d *Dashboard) buildIndicator(page string) string {
	return fmt.Sprintf("%s%s%s%s | %s%s%s%s | %s%s%s%s | %s%s%s%s | %s%s%s%s | %s%s%s%s | %s%s%s%s | %s%s%s%s",
		theme.ColorFor(page == "workloads"), tview.Escape("[w]"), theme.Reset, "orkloads",
		theme.ColorFor(page == "network"), tview.Escape("[n]"), theme.Reset, "etwork",
		theme.ColorFor(page == "cluster"), tview.Escape("[c]"), theme.Reset, "luster",
		theme.ColorFor(page == "metrics"), tview.Escape("[m]"), theme.Reset, "etrics",
		theme.Header, tview.Escape("[a]"), theme.Reset, "lerts",
		theme.Header, tview.Escape("[e]"), theme.Reset, "vents",
		theme.Header, tview.Escape("[0-9]"), theme.Reset, " namespace",
		theme.Header, tview.Escape("[q]"), theme.Reset, "uit")
}

func (d *Dashboard) applyBrowseStyle(box *tview.TextView) {
	d.originalTitles[box] = box.GetTitle()
	box.SetBorderColor(tcell.ColorGreen)
	box.SetTitle(tview.Escape(fmt.Sprintf("%s [L]ogs / [D]escribe", strings.TrimSpace(d.originalTitles[box]))))
}

func (d *Dashboard) restoreBrowseStyle(box *tview.TextView) {
	if raw, ok := d.contentCache[box]; ok {
		box.SetText(raw)
	}
	if title, ok := d.originalTitles[box]; ok {
		box.SetTitle(title)
	}
	if def, ok := d.borderDefaults[box]; ok {
		box.SetBorderColor(def)
	}
}

func (d *Dashboard) openLogsSelected() {
	if d.browseBox == nil || d.browseBox != d.podsView {
		return
	}
	raw := d.contentCache[d.browseBox]
	lines := strings.Split(raw, "\n")
	if d.selectedLine < 0 || d.selectedLine >= len(lines) {
		return
	}
	nsTarget := d.resourceNSFor(d.browseBox, lines[d.selectedLine], d.ns)
	name := d.resourceNameFor(d.browseBox, lines[d.selectedLine], d.ns)
	if name != "" {
		d.openLogs(name, nsTarget)
	}
}

func (d *Dashboard) openDescribeSelected() {
	if d.browseBox == nil {
		return
	}
	raw := d.contentCache[d.browseBox]
	lines := strings.Split(raw, "\n")
	if d.selectedLine < 0 || d.selectedLine >= len(lines) {
		return
	}
	kind := d.resourceKindFor(d.browseBox, lines, d.selectedLine)
	if kind == "" {
		return
	}
	nsTarget := d.resourceNSFor(d.browseBox, lines[d.selectedLine], d.ns)
	name := d.resourceNameFor(d.browseBox, lines[d.selectedLine], d.ns)
	if name != "" {
		d.openDescribe(kind, name, nsTarget)
	}
}

func (d *Dashboard) setPage(page string) {
	d.exitBrowse()
	d.currentPage = page
	d.pages.SwitchToPage(page)
	d.pageIndicator.SetText(d.buildIndicator(page))
	if items := d.focusOrder(); len(items) > 0 {
		d.app.SetFocus(items[0])
	}
	d.highlightFocus()
}

func (d *Dashboard) moveFocus(delta int) {
	list := d.focusOrder()
	if len(list) == 0 {
		return
	}
	cur := d.app.GetFocus()
	idx := -1
	for i, v := range list {
		if v == cur {
			idx = i
			break
		}
	}
	if idx == -1 {
		d.app.SetFocus(list[0])
		d.highlightFocus()
		return
	}
	next := (idx + delta + len(list)) % len(list)
	d.app.SetFocus(list[next])
	d.highlightFocus()
}

func (d *Dashboard) highlightBox(box *tview.TextView) {
	raw, ok := d.contentCache[box]
	if !ok || raw == "" {
		return
	}
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return
	}
	if !d.isSelectable(box, lines, d.selectedLine) {
		d.selectedLine = d.findSelectable(box, lines, d.selectedLine, 1, true)
	}
	if d.selectedLine == -1 {
		box.SetText(raw)
		return
	}
	for i, l := range lines {
		if i == d.selectedLine {
			lines[i] = fmt.Sprintf("[black:yellow]%s[-:-:-]", l)
		}
	}
	box.SetText(strings.Join(lines, "\n"))
	if d.selectedLine >= 0 {
		_, currentTop := box.GetScrollOffset()
		_, _, _, h := box.GetInnerRect()
		if h <= 0 {
			return
		}
		if d.selectedLine < currentTop {
			box.ScrollTo(d.selectedLine, 0)
		} else if d.selectedLine >= currentTop+h {
			box.ScrollTo(d.selectedLine-h+1, 0)
		}
	}
}

func (d *Dashboard) enterBrowse(box *tview.TextView) {
	if box != d.podsView && box != d.workloadsView && box != d.configView && box != d.networkView && box != d.storageView && box != d.infraView {
		return
	}
	raw := d.contentCache[box]
	lines := strings.Split(raw, "\n")
	start := d.findSelectable(box, lines, 1, 1, true)
	if start == -1 {
		return
	}
	d.browseBox = box
	d.selectedLine = start
	d.applyBrowseStyle(box)
	d.app.SetFocus(box)
	d.highlightBox(box)
}

func (d *Dashboard) exitBrowse() {
	if d.browseBox != nil {
		d.restoreBrowseStyle(d.browseBox)
	}
	d.browseBox = nil
}

func (d *Dashboard) switchPage(delta int) {
	idx := 0
	for i, p := range d.pageOrder {
		if p == d.currentPage {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(d.pageOrder)) % len(d.pageOrder)
	d.setPage(d.pageOrder[idx])
}

func (d *Dashboard) openLogs(name, targetNS string) {
	if name == "" {
		return
	}
	d.openModal("LOGS "+name, "Carregando logs...")

	go func() {
		nsUse := d.ns
		if strings.TrimSpace(targetNS) != "" {
			nsUse = targetNS
		}
		args := []string{"logs", name, "--tail=200"}
		args = append(args, data.NSSelector(nsUse, false)...)
		logs := data.RunKubectl(args...)
		_ = d.app.QueueUpdateDraw(func() {
			d.modalLogs.SetText(logs)
		})
	}()
}

func (d *Dashboard) openDescribe(kind, name, targetNS string) {
	if name == "" || kind == "" {
		return
	}
	d.openModal(fmt.Sprintf("DESCRIBE %s/%s", kind, name), "Carregando describe...")

	go func() {
		nsUse := d.ns
		if strings.TrimSpace(targetNS) != "" {
			nsUse = targetNS
		}
		args := []string{"describe", kind, name}
		args = append(args, data.NSSelector(nsUse, false)...)
		desc := data.RunKubectl(args...)
		_ = d.app.QueueUpdateDraw(func() {
			d.modalLogs.SetText(desc)
		})
	}()
}

func (d *Dashboard) openModal(title, body string) {
	d.restoreFocus = d.app.GetFocus()
	d.modalOpen = true
	d.modalLogs.SetTitle(title + " (Esc fecha)")
	d.modalLogs.SetText(body)
	d.pages.AddPage("modalLogs", tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.modalLogs, 0, 1, true), true, true)
}

func (d *Dashboard) showInfo(msg string) {
	_ = d.app.QueueUpdateDraw(func() {
		d.pages.RemovePage("infoPopup")
		d.infoPopup.SetText(msg)
		d.pages.AddPage("infoPopup", tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(d.infoPopup, 5, 0, true), true, true)
	})
	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = d.app.QueueUpdateDraw(func() {
			d.pages.RemovePage("infoPopup")
		})
	}()
}

func (d *Dashboard) findSelectable(box *tview.TextView, lines []string, start int, delta int, wrap bool) int {
	if len(lines) == 0 || delta == 0 {
		return -1
	}
	if wrap {
		i := start
		for count := 0; count < len(lines); count++ {
			if i < 0 {
				i = len(lines) - 1
			}
			if i >= len(lines) {
				i = 0
			}
			if d.isSelectable(box, lines, i) {
				return i
			}
			i += delta
		}
		return -1
	}
	for i := start; i >= 0 && i < len(lines); i += delta {
		if d.isSelectable(box, lines, i) {
			return i
		}
	}
	return -1
}

func (d *Dashboard) isSelectable(box *tview.TextView, lines []string, idx int) bool {
	if idx < 0 || idx >= len(lines) {
		return false
	}
	line := strings.TrimSpace(lines[idx])
	if line == "" {
		return false
	}
	if strings.ToUpper(line) == line {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	joined := strings.ToLower(strings.Join(fields, " "))
	if strings.Contains(joined, "no resources found") {
		return false
	}
	if fields[0] == "NAME" {
		return false
	}
	if strings.ToUpper(fields[0]) == fields[0] && len(fields[0]) <= 8 && len(fields) == 1 {
		return false
	}
	switch box {
	case d.workloadsView:
		if len(fields) <= 1 {
			return false
		}
		return true
	case d.podsView:
		return len(fields) > 1
	case d.alertsView, d.eventsView:
		return len(fields) > 0
	default:
		return len(fields) > 1
	}
}

func (d *Dashboard) resourceKindFor(box *tview.TextView, lines []string, idx int) string {
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	switch box {
	case d.podsView, d.alertsView:
		return "pod"
	case d.workloadsView:
		for i := idx; i >= 0; i-- {
			title := strings.TrimSpace(lines[i])
			switch title {
			case "DEPLOY":
				return "deploy"
			case "RS":
				return "rs"
			case "STS":
				return "sts"
			case "DS":
				return "ds"
			case "JOBS":
				return "jobs"
			case "CRONJOBS":
				return "cronjobs"
			}
		}
		return ""
	case d.configView:
		for i := idx; i >= 0; i-- {
			title := strings.TrimSpace(lines[i])
			switch title {
			case "SECRETS":
				return "secrets"
			case "CONFIGMAPS":
				return "configmaps"
			case "SERVICEACCOUNTS":
				return "serviceaccounts"
			}
		}
		return ""
	case d.networkView:
		for i := idx; i >= 0; i-- {
			title := strings.TrimSpace(lines[i])
			switch title {
			case "SVC":
				return "svc"
			case "INGRESS":
				return "ingress"
			case "ENDPOINTS":
				return "endpoints"
			}
		}
		return ""
	case d.storageView:
		for i := idx; i >= 0; i-- {
			title := strings.TrimSpace(lines[i])
			switch title {
			case "PVC":
				return "pvc"
			case "PV":
				return "pv"
			}
		}
		return ""
	case d.infraView:
		for i := idx; i >= 0; i-- {
			title := strings.TrimSpace(lines[i])
			switch title {
			case "NODES":
				return "nodes"
			case "CRDS":
				return "crd"
			}
		}
		return ""
	default:
		return ""
	}
}

func (d *Dashboard) resourceNSFor(box *tview.TextView, line string, currentNS string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	nsTrim := strings.TrimSpace(currentNS)
	isAll := nsTrim == "" || strings.EqualFold(nsTrim, "all")
	if isAll {
		return fields[0]
	}
	return nsTrim
}

func (d *Dashboard) resourceNameFor(box *tview.TextView, line string, currentNS string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	nsTrim := strings.TrimSpace(currentNS)
	isAll := nsTrim == "" || strings.EqualFold(nsTrim, "all")
	if box == d.infraView || (box == d.storageView && len(fields) > 0) {
		return fields[0]
	}
	if isAll {
		if len(fields) >= 2 {
			return fields[1]
		}
		return ""
	}
	return fields[0]
}

func (d *Dashboard) adjustSelection(delta int) {
	if d.browseBox == nil {
		return
	}
	raw := d.contentCache[d.browseBox]
	lines := strings.Split(raw, "\n")
	next := d.findSelectable(d.browseBox, lines, d.selectedLine+delta, delta, false)
	if next != -1 {
		d.selectedLine = next
	}
	d.highlightBox(d.browseBox)
}

func (d *Dashboard) update() {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()

	currentNS := d.ns

	summary := data.BuildSummary(currentNS, d.clientset)
	nsView, nsNames := data.BuildNamespaces()
	alerts := data.BuildAlerts(currentNS)
	cfg := data.BuildConfigGroup(currentNS)
	net := data.BuildNetworkGroup(currentNS)
	storage := data.BuildStorageGroup(currentNS)
	infra := data.BuildInfraGroup()
	wl := data.BuildWorkloadsGroup(currentNS)
	pods := dataClampLines(data.BuildPods(currentNS), 30)
	metrics := data.BuildMetrics(currentNS)
	events := data.BuildEvents(currentNS)

	_ = d.app.QueueUpdateDraw(func() {
		d.contentCache[d.namespacesView] = nsView
		d.contentCache[d.overview] = summary
		d.contentCache[d.alertsView] = alerts
		d.contentCache[d.configView] = cfg
		d.contentCache[d.networkView] = net
		d.contentCache[d.storageView] = storage
		d.contentCache[d.infraView] = infra
		d.contentCache[d.workloadsView] = wl
		d.contentCache[d.podsView] = pods
		d.contentCache[d.metricsView] = metrics
		d.contentCache[d.eventsView] = events
		d.nsList = nsNames

		if strings.TrimSpace(alerts) == "" {
			d.borderDefaults[d.alertsView] = tcell.ColorGreen
		} else {
			d.borderDefaults[d.alertsView] = tcell.ColorYellow
		}
		if strings.TrimSpace(events) == "" {
			d.borderDefaults[d.eventsView] = tcell.ColorGreen
		} else {
			d.borderDefaults[d.eventsView] = tcell.ColorLightCyan
		}
		d.borderDefaults[d.overview] = tcell.ColorLightSkyBlue

		if d.browseBox != nil {
			d.highlightBox(d.browseBox)
		} else {
			d.namespacesView.SetText(nsView)
			d.overview.SetText(summary)
			d.alertsView.SetText(alerts)
			d.configView.SetText(cfg)
			d.networkView.SetText(net)
			d.storageView.SetText(storage)
			d.infraView.SetText(infra)
			d.workloadsView.SetText(wl)
			d.podsView.SetText(pods)
			d.metricsView.SetText(metrics)
			d.eventsView.SetText(events)
		}

		adjust := func(f *tview.Flex, item tview.Primitive, hasContent bool, minHeight int, keepBorder bool) {
			if hasContent {
				f.ResizeItem(item, 0, 1)
				if box, ok := item.(*tview.TextView); ok {
					box.SetBorder(true)
					if strings.TrimSpace(d.contentCache[box]) == "" {
						box.SetText("")
					}
					box.SetBorderColor(tcell.ColorWhite)
				}
			} else {
				if minHeight > 0 {
					f.ResizeItem(item, minHeight, 0)
				} else {
					f.ResizeItem(item, 1, 0)
				}
				if box, ok := item.(*tview.TextView); ok {
					box.SetText("")
					box.SetBorder(keepBorder)
					if keepBorder {
						box.SetBorderColor(tcell.ColorGray)
					}
				}
			}
		}

		adjust(d.clusterPage, d.infraView, strings.TrimSpace(d.contentCache[d.infraView]) != "", 3, true)
		adjust(d.clusterPage, d.configView, strings.TrimSpace(d.contentCache[d.configView]) != "", 3, true)
		adjust(d.clusterPage, d.storageView, strings.TrimSpace(d.contentCache[d.storageView]) != "", 3, true)
		adjust(d.workloadsPage, d.workloadsView, strings.TrimSpace(d.contentCache[d.workloadsView]) != "", 4, true)
		adjust(d.workloadsPage, d.podsView, strings.TrimSpace(d.contentCache[d.podsView]) != "", 4, true)
		adjust(d.workloadsPage, d.metricsView, strings.TrimSpace(d.contentCache[d.metricsView]) != "", 3, true)
		adjust(d.networkPage, d.networkView, strings.TrimSpace(d.contentCache[d.networkView]) != "", 3, true)
		adjust(d.metricsPage, d.metricsView, strings.TrimSpace(d.contentCache[d.metricsView]) != "", 3, true)

		if d.app.GetFocus() == nil {
			if items := d.focusOrder(); len(items) > 0 {
				d.app.SetFocus(items[0])
			}
		}
		d.highlightFocus()
	})
}

func (d *Dashboard) scheduleUpdate() {
	select {
	case d.updateCh <- struct{}{}:
	default:
	}
}

func (d *Dashboard) handleInput(ev *tcell.EventKey) *tcell.EventKey {
	if d.modalOpen {
		if ev.Key() == tcell.KeyEsc {
			d.pages.RemovePage("modalLogs")
			d.modalOpen = false
			if d.restoreFocus != nil {
				d.app.SetFocus(d.restoreFocus)
			}
			return nil
		}
		return ev
	}

	if d.browseBox != nil {
		switch ev.Key() {
		case tcell.KeyUp:
			d.adjustSelection(-1)
			return nil
		case tcell.KeyDown:
			d.adjustSelection(1)
			return nil
		case tcell.KeyEnter:
			d.openLogsSelected()
			return nil
		case tcell.KeyEsc:
			d.exitBrowse()
			return nil
		}
	}

	switch {
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'w':
		d.setPage("workloads")
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'n':
		d.setPage("network")
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'c':
		d.setPage("cluster")
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'm':
		d.setPage("metrics")
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'l':
		d.openLogsSelected()
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() >= '0' && ev.Rune() <= '9':
		idx := int(ev.Rune() - '0')
		if idx >= 0 && idx < len(d.nsList) {
			oldNS := d.ns
			d.ns = strings.TrimSpace(d.nsList[idx])
			statusMsg := fmt.Sprintf("Namespace: %s -> %s", displayNS(oldNS), displayNS(d.ns))
			go d.showInfo(statusMsg)
			d.exitBrowse()
			d.scheduleUpdate()
		}
		return nil
	case ev.Key() == tcell.KeyLeft:
		d.switchPage(-1)
		return nil
	case ev.Key() == tcell.KeyRight:
		d.switchPage(1)
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'a':
		content := d.contentCache[d.alertsView]
		if strings.TrimSpace(content) == "" {
			content = "Sem alertas."
		}
		d.openModal("ALERTAS", content)
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'e':
		content := d.contentCache[d.eventsView]
		if strings.TrimSpace(content) == "" {
			content = "Sem eventos."
		}
		d.openModal("EVENTS", content)
		return nil
	case ev.Key() == tcell.KeyUp:
		d.moveFocus(-1)
		return nil
	case ev.Key() == tcell.KeyDown:
		d.moveFocus(1)
		return nil
	case ev.Key() == tcell.KeyEnter:
		if d.browseBox == nil {
			if tv, ok := d.app.GetFocus().(*tview.TextView); ok {
				d.enterBrowse(tv)
			}
			return nil
		}
		d.openLogsSelected()
	case ev.Key() == tcell.KeyEsc:
		if d.browseBox != nil {
			d.exitBrowse()
			return nil
		}
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'q':
		d.app.Stop()
		return nil
	case ev.Key() == tcell.KeyRune && ev.Rune() == 'd':
		if d.browseBox != nil {
			d.openDescribeSelected()
			return nil
		}
	}
	return ev
}

func (d *Dashboard) captureInterrupt() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		d.app.Stop()
	}()
}

func (d *Dashboard) Run() error {
	d.setPage(d.currentPage)
	d.scheduleUpdate()

	go func() {
		for range d.updateCh {
			d.update()
		}
	}()

	d.ticker = time.NewTicker(2 * time.Second)
	defer d.ticker.Stop()
	go func() {
		for range d.ticker.C {
			d.scheduleUpdate()
		}
	}()

	d.captureInterrupt()
	d.app.SetInputCapture(d.handleInput)
	return d.app.SetRoot(d.root, true).EnableMouse(true).Run()
}

func newTextArea(title string) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false).
		SetWrap(true).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(title)
	return tv
}

func newBox(title string) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false).
		SetWrap(false).
		SetScrollable(true)
	tv.SetBorder(true).SetTitle(title)
	return tv
}

func dataClampLines(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func displayNS(ns string) string {
	if strings.TrimSpace(ns) == "" {
		return "ALL"
	}
	return ns
}
