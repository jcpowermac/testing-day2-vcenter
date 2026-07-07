package framework

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ScrapeOperatorMetrics finds a Running pod matching the label selector in the
// given namespace and scrapes its /metrics endpoint via the Kubernetes API
// server proxy. It returns the raw Prometheus text-format metrics body.
func ScrapeOperatorMetrics(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	pods, err := ListPodsByLabel(ctx, client, namespace, labelSelector)
	if err != nil {
		return "", fmt.Errorf("scrape metrics: %w", err)
	}

	var podName string
	for i := range pods {
		if pods[i].Status.Phase == corev1.PodRunning {
			podName = pods[i].Name
			break
		}
	}
	if podName == "" {
		return "", fmt.Errorf("scrape metrics: no Running pod found in %s with selector %q", namespace, labelSelector)
	}

	body, err := client.CoreV1().
		Pods(namespace).
		ProxyGet("https", podName, "8445", "/metrics", nil).
		DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("scrape metrics from pod %s: %w", podName, err)
	}

	return string(body), nil
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

		// Split "metric_name{labels} value" or "metric_name value" into name part and value.
		// The value is always the last whitespace-separated token.
		name, value, ok := splitMetricLine(line)
		if !ok {
			continue
		}

		// Separate the bare metric name from the optional {labels} block.
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
	// Handle "{...} value" — find the closing brace first.
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
		// Strip surrounding quotes.
		val = strings.Trim(val, "\"")
		result[key] = val
	}
	return result
}
