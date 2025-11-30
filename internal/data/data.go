package data

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"ktwins/internal/theme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	cmdTimeout     = 1200 * time.Millisecond
	maxOutputBytes = 64 * 1024 // protege UI de outputs gigantes
)

// Executa comandos (kubectl) com timeout e limite de saída; evita shell para reduzir overhead/injeção.
func runWithTimeout(timeout time.Duration, cmd string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c := exec.CommandContext(ctx, cmd, args...)
	var buf strings.Builder
	c.Stdout = newLimitedWriter(&buf, maxOutputBytes)
	c.Stderr = newLimitedWriter(&buf, maxOutputBytes)

	_ = c.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	return buf.String()
}

func runKubectl(args ...string) string {
	full := append([]string{"--request-timeout=1s"}, args...)
	return runWithTimeout(cmdTimeout, "kubectl", full...)
}

// RunKubectl exported for UI actions (logs/describe).
func RunKubectl(args ...string) string {
	return runKubectl(args...)
}

// limitador de saída para proteger a UI
type limitedWriter struct {
	w     *strings.Builder
	limit int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	avail := lw.limit - lw.w.Len()
	if avail <= 0 {
		return len(p), nil
	}
	if len(p) > avail {
		p = p[:avail]
	}
	return lw.w.Write(p)
}

func newLimitedWriter(dst *strings.Builder, limit int) *limitedWriter {
	return &limitedWriter{w: dst, limit: limit}
}

// Contador genérico (usando client-go + kubectl para CRD)
func count(c *kubernetes.Clientset, kind, ns string, cluster bool) int {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	nsTarget := strings.TrimSpace(ns)
	if nsTarget == "" || strings.EqualFold(nsTarget, "all") {
		nsTarget = metav1.NamespaceAll
	}

	switch kind {
	case "deploy":
		out, err := c.AppsV1().Deployments(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "rs":
		out, err := c.AppsV1().ReplicaSets(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "sts":
		out, err := c.AppsV1().StatefulSets(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "ds":
		out, err := c.AppsV1().DaemonSets(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "jobs":
		out, err := c.BatchV1().Jobs(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "cronjobs":
		out, err := c.BatchV1().CronJobs(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "pods":
		out, err := c.CoreV1().Pods(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "svc":
		out, err := c.CoreV1().Services(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "ingress":
		out, err := c.NetworkingV1().Ingresses(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "endpoints":
		out, err := c.CoreV1().Endpoints(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "pvc":
		out, err := c.CoreV1().PersistentVolumeClaims(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "secrets":
		out, err := c.CoreV1().Secrets(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "configmaps":
		out, err := c.CoreV1().ConfigMaps(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "serviceaccounts":
		out, err := c.CoreV1().ServiceAccounts(nsTarget).List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "nodes":
		out, err := c.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "pv":
		out, err := c.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil || out == nil {
			return 0
		}
		return len(out.Items)
	case "crd":
		crdOut := runKubectl("get", "crd", "--no-headers")
		if strings.Contains(crdOut, "No resources found") {
			return 0
		}
		lines := strings.Split(strings.TrimSpace(crdOut), "\n")
		if len(lines) == 0 || lines[0] == "" {
			return 0
		}
		return len(lines)
	}

	return 0
}

func BuildSummary(ns string, c *kubernetes.Clientset) string {
	displayNS := ns
	if displayNS == "" {
		displayNS = "ALL"
	}
	deploy := count(c, "deploy", ns, false)
	rs := count(c, "rs", ns, false)
	sts := count(c, "sts", ns, false)
	ds := count(c, "ds", ns, false)
	jobs := count(c, "jobs", ns, false)
	cj := count(c, "cronjobs", ns, false)
	pods := count(c, "pods", ns, false)
	svc := count(c, "svc", ns, false)
	ing := count(c, "ingress", ns, false)
	ep := count(c, "endpoints", ns, false)
	pvc := count(c, "pvc", ns, false)
	pv := count(c, "pv", ns, true)
	secrets := count(c, "secrets", ns, false)
	cm := count(c, "configmaps", ns, false)
	sa := count(c, "serviceaccounts", ns, false)
	nodes := count(c, "nodes", ns, true)
	crd := count(c, "crd", ns, true)

	line1 := fmt.Sprintf("%sNS%s %-10s",
		theme.Title, theme.Reset, displayNS)

	line2 := fmt.Sprintf("%sinfra%s nodes:%d crd:%d",
		theme.Header, theme.Reset, nodes, crd)

	line3 := fmt.Sprintf("%sconfig%s sec:%d cm:%d sa:%d",
		theme.Header, theme.Reset, secrets, cm, sa)

	line4 := fmt.Sprintf("%snet%s svc:%d ing:%d ep:%d",
		theme.Header, theme.Reset, svc, ing, ep)

	line5 := fmt.Sprintf("%sstorage%s pvc:%d pv:%d",
		theme.Header, theme.Reset, pvc, pv)

	line6 := fmt.Sprintf("%sworkloads%s d:%d rs:%d sts:%d ds:%d jobs:%d cj:%d",
		theme.Header, theme.Reset, deploy, rs, sts, ds, jobs, cj)

	line7 := fmt.Sprintf("%spods%s %d",
		theme.Header, theme.Reset, pods)

	return strings.Join([]string{
		line1,
		line2,
		line3,
		line4,
		line5,
		line6,
		line7,
	}, "\n")

}

func BuildAlerts(ns string) string {
	args := append([]string{"get", "pods", "--no-headers"}, NSSelector(ns, true)...)
	out := runKubectl(args...)
	lines := strings.Split(out, "\n")
	var alerts strings.Builder
	count := 0

	for _, l := range lines {
		if l == "" {
			continue
		}
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		name := f[0]
		status := f[2]

		switch status {
		case "CrashLoopBackOff", "Error", "ImagePullBackOff",
			"ErrImagePull", "Pending", "CreateContainerError":
			alerts.WriteString(fmt.Sprintf("%s⚠ %s: %s%s\n",
				theme.Red, name, status, theme.Reset))
			count++
			if count >= 5 {
				break
			}
		}
	}
	return strings.TrimSpace(alerts.String())
}

func buildInfra() string {
	return strings.TrimSpace(runKubectl("get", "nodes")) + "\n"
}

func buildCRD() string {
	return strings.TrimSpace(runKubectl("get", "crd")) + "\n"
}

func buildConfig(ns, kind string) string {
	args := []string{"get", kind}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	if strings.Contains(out, "No resources found") {
		return ""
	}
	return out
}

func buildNetwork(ns, kind string) string {
	args := []string{"get", kind}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	if strings.Contains(out, "No resources found") {
		return ""
	}
	return out
}

func buildStorage(ns string) string {
	args := []string{"get", "pvc"}
	args = append(args, NSSelector(ns, true)...)
	pvc := runKubectl(args...)
	if strings.Contains(pvc, "No resources found") {
		return ""
	}
	return pvc
}

func buildPV() string {
	pv := runKubectl("get", "pv")
	if strings.Contains(pv, "No resources found") {
		return ""
	}
	return pv
}

func buildWorkloads(ns, kind string) string {
	args := []string{"get", kind}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	if strings.Contains(out, "No resources found") {
		return ""
	}
	return out
}

func BuildPods(ns string) string {
	args := []string{"get", "pods"}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	if strings.Contains(out, "No resources found") {
		return ""
	}
	return out
}

func BuildMetrics(ns string) string {
	args := []string{"top", "pods"}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	if strings.TrimSpace(out) == "" {
		return ""
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "error") || strings.Contains(lower, "timeout") || strings.Contains(lower, "unavailable") {
		return ""
	}
	return theme.Green + out + theme.Reset
}

func BuildEvents(ns string) string {
	args := []string{"get", "events", "--sort-by=.metadata.creationTimestamp"}
	args = append(args, NSSelector(ns, true)...)
	out := runKubectl(args...)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func BuildConfigGroup(ns string) string {
	return buildGroup(ns, []string{"secrets", "configmaps", "serviceaccounts"}, buildConfig)
}

func BuildNetworkGroup(ns string) string {
	return buildGroup(ns, []string{"svc", "ingress", "endpoints"}, buildNetwork)
}

func BuildStorageGroup(ns string) string {
	var b strings.Builder
	if pvcOut := buildStorage(ns); pvcOut != "" {
		fmt.Fprintf(&b, "PVC\n%s\n\n", pvcOut)
	}
	if pvOut := buildPV(); pvOut != "" {
		fmt.Fprintf(&b, "PV\n%s\n\n", pvOut)
	}
	return strings.TrimSpace(b.String())
}

func BuildInfraGroup() string {
	var b strings.Builder
	if nodes := strings.TrimSpace(buildInfra()); nodes != "" {
		fmt.Fprintf(&b, "NODES\n%s\n\n", nodes)
	}
	if crd := strings.TrimSpace(buildCRD()); crd != "" {
		fmt.Fprintf(&b, "CRDs\n%s\n\n", crd)
	}
	return strings.TrimSpace(b.String())
}

func BuildWorkloadsGroup(ns string) string {
	var b strings.Builder
	for _, kind := range []string{"deploy", "rs", "sts", "ds", "jobs", "cronjobs"} {
		if out := buildWorkloads(ns, kind); out != "" {
			fmt.Fprintf(&b, "%s\n%s\n\n", strings.ToUpper(kind), clampLines(out, 20))
		}
	}
	return strings.TrimSpace(b.String())
}

func buildGroup(ns string, kinds []string, fetch func(ns, kind string) string) string {
	var b strings.Builder
	for _, kind := range kinds {
		if out := fetch(ns, kind); out != "" {
			fmt.Fprintf(&b, "%s\n%s\n\n", strings.ToUpper(kind), out)
		}
	}
	return strings.TrimSpace(b.String())
}

func NSSelector(ns string, allowAll bool) []string {
	trimmed := strings.TrimSpace(ns)
	if trimmed == "" && allowAll {
		return []string{"-A"}
	}
	if trimmed == "" {
		return nil
	}
	if allowAll && strings.EqualFold(trimmed, "all") {
		return []string{"-A"}
	}
	return []string{"-n", trimmed}
}

func BuildNamespaces() (string, []string) {
	out := runKubectl("get", "ns", "--no-headers")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var b strings.Builder
	names := []string{""}
	b.WriteString("0) ALL\n")
	idx := 1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		names = append(names, name)
		fmt.Fprintf(&b, "%d) %s\n", idx, name)
		idx++
	}
	return b.String(), names
}

func clampLines(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxLines], "\n")
}
