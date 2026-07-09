package framework

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ScrapeOperatorMetrics queries the cluster's Thanos endpoint for all metrics
// emitted by the operator and returns them in Prometheus text exposition format
// so that ParseMetricValue continues to work unchanged.
//
// The operator's metrics endpoint requires mTLS (via kube-rbac-proxy), which
// makes Pod ProxyGet unusable. Instead we read the already-scraped data from
// the in-cluster Thanos instance.
func ScrapeOperatorMetrics(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		cfg, err = loadRestConfig()
		if err != nil {
			return "", fmt.Errorf("scrape metrics: build rest config: %w", err)
		}
	}

	thanosHost, err := thanosQuerierHost(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("scrape metrics: %w", err)
	}

	token, err := prometheusToken(ctx, client)
	if err != nil {
		return "", fmt.Errorf("scrape metrics: %w", err)
	}

	job := jobNameFromLabelSelector(labelSelector)
	query := fmt.Sprintf(`{job="%s"}`, job)
	raw, err := queryThanos(ctx, thanosHost, token, query)
	if err != nil {
		return "", fmt.Errorf("scrape metrics: %w", err)
	}

	text, err := vectorToText(raw)
	if err != nil {
		return "", fmt.Errorf("scrape metrics: %w", err)
	}
	return text, nil
}

// thanosQuerierHost returns the host of the thanos-querier route.
func thanosQuerierHost(ctx context.Context, cfg *rest.Config) (string, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("dynamic client: %w", err)
	}
	routeGVR := schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
	obj, err := dyn.Resource(routeGVR).Namespace("openshift-monitoring").Get(ctx, "thanos-querier", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get thanos-querier route: %w", err)
	}
	host, found, err := unstructuredString(obj.Object, "spec", "host")
	if err != nil || !found || host == "" {
		return "", fmt.Errorf("thanos-querier route has no spec.host")
	}
	return host, nil
}

func unstructuredString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	cur := obj
	for i, f := range fields {
		v, ok := cur[f]
		if !ok {
			return "", false, nil
		}
		if i == len(fields)-1 {
			s, ok := v.(string)
			return s, ok, nil
		}
		next, ok := v.(map[string]interface{})
		if !ok {
			return "", false, fmt.Errorf("field %q is not a map", f)
		}
		cur = next
	}
	return "", false, nil
}

// prometheusToken creates a short-lived token for the prometheus-k8s SA.
func prometheusToken(ctx context.Context, client kubernetes.Interface) (string, error) {
	expiry := int64(600)
	tr, err := client.CoreV1().ServiceAccounts("openshift-monitoring").CreateToken(
		ctx,
		"prometheus-k8s",
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &expiry,
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("create prometheus-k8s token: %w", err)
	}
	return tr.Status.Token, nil
}

// jobNameFromLabelSelector converts "name=vmware-vsphere-csi-driver-operator"
// into the Prometheus job name "vmware-vsphere-csi-driver-operator-metrics".
func jobNameFromLabelSelector(selector string) string {
	parts := strings.SplitN(selector, "=", 2)
	name := selector
	if len(parts) == 2 {
		name = parts[1]
	}
	return name + "-metrics"
}

// queryThanos hits the Thanos querier /api/v1/query endpoint.
func queryThanos(ctx context.Context, host, token, query string) (json.RawMessage, error) {
	u := fmt.Sprintf("https://%s/api/v1/query", host)
	form := url.Values{"query": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thanos query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read thanos response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thanos returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode thanos response: %w", err)
	}
	if parsed.Status != "success" {
		return nil, fmt.Errorf("thanos query status: %s", parsed.Status)
	}
	return parsed.Data, nil
}

// vectorToText converts a Thanos instant-query vector result into Prometheus
// text exposition format so that ParseMetricValue works unchanged.
func vectorToText(data json.RawMessage) (string, error) {
	var envelope struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("decode vector: %w", err)
	}

	var b strings.Builder
	for _, r := range envelope.Result {
		name := r.Metric["__name__"]
		delete(r.Metric, "__name__")

		var labels []string
		for k, v := range r.Metric {
			labels = append(labels, fmt.Sprintf(`%s="%s"`, k, v))
		}

		val := "0"
		if len(r.Value) == 2 {
			if s, ok := r.Value[1].(string); ok {
				val = s
			}
		}

		if len(labels) > 0 {
			fmt.Fprintf(&b, "%s{%s} %s\n", name, strings.Join(labels, ","), val)
		} else {
			fmt.Fprintf(&b, "%s %s\n", name, val)
		}
	}
	return b.String(), nil
}

// ParseMetricValue searches Prometheus text-format metrics for a line matching
// metricName. If labels is non-nil, the line must also contain every key="value"
// pair from the map inside the {...} label block. Returns the parsed float64
// value from the first matching line.
//
// Example input line:
//
//	vsphere_csi_tag_operations_total{operation="create",status="success"} 42
func ParseMetricValue(metricsText, metricName string, labels map[string]string) (float64, error) {
	for _, line := range strings.Split(metricsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, ok := splitMetricLine(line)
		if !ok {
			continue
		}

		bareName, labelBlock := splitNameAndLabels(name)
		if bareName != metricName {
			continue
		}

		if labels != nil && !labelsMatch(labelBlock, labels) {
			continue
		}

		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("parse value %q for metric %s: %w", value, metricName, err)
		}
		return v, nil
	}
	return 0, fmt.Errorf("metric %q not found", metricName)
}

// splitMetricLine splits a Prometheus line into the name part (including
// optional labels) and the value string. Returns false if the line does not
// have the expected format.
func splitMetricLine(line string) (name, value string, ok bool) {
	if idx := strings.Index(line, "}"); idx != -1 {
		rest := strings.TrimSpace(line[idx+1:])
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			return "", "", false
		}
		return line[:idx+1], parts[0], true
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// splitNameAndLabels separates "metric_name{k=v,...}" into "metric_name" and
// "k=v,...". If there are no labels, labelBlock is empty.
func splitNameAndLabels(name string) (bareName, labelBlock string) {
	if idx := strings.Index(name, "{"); idx != -1 {
		bareName = name[:idx]
		labelBlock = strings.TrimSuffix(name[idx+1:], "}")
		return bareName, labelBlock
	}
	return name, ""
}

// labelsMatch checks whether every required key="value" pair appears in the
// Prometheus label block string (comma-separated key="value" pairs).
func labelsMatch(labelBlock string, required map[string]string) bool {
	parsed := parseLabelBlock(labelBlock)
	for k, v := range required {
		if parsed[k] != v {
			return false
		}
	}
	return true
}

// parseLabelBlock parses a Prometheus label block like
// `operation="create",status="success"` into a map.
func parseLabelBlock(block string) map[string]string {
	result := make(map[string]string)
	if block == "" {
		return result
	}
	for _, pair := range strings.Split(block, ",") {
		pair = strings.TrimSpace(pair)
		eqIdx := strings.Index(pair, "=")
		if eqIdx < 0 {
			continue
		}
		key := pair[:eqIdx]
		val := pair[eqIdx+1:]
		val = strings.Trim(val, "\"")
		result[key] = val
	}
	return result
}
