package config

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// TaintSelector matches a node taint by key, with an optionally-specified value and
// effect; an unspecified value or effect acts as a wildcard for that part. This
// mirrors kubectl's own "key=value:effect" taint syntax (with value and/or effect
// omittable), so operators already familiar with `kubectl taint` can reuse it.
type TaintSelector struct {
	Key       string
	Value     string
	HasValue  bool
	Effect    corev1.TaintEffect
	HasEffect bool
}

// Matches reports whether t satisfies this selector.
func (ts TaintSelector) Matches(t corev1.Taint) bool {
	return t.Key == ts.Key &&
		(!ts.HasValue || t.Value == ts.Value) &&
		(!ts.HasEffect || t.Effect == ts.Effect)
}

func parseTaintSelector(s string) (TaintSelector, error) {
	key := s
	var ts TaintSelector

	if idx := strings.LastIndex(key, ":"); idx != -1 {
		ts.Effect = corev1.TaintEffect(key[idx+1:])
		ts.HasEffect = true
		key = key[:idx]
		if ts.Effect == "" {
			return TaintSelector{}, fmt.Errorf("empty effect in taint selector %q", s)
		}
	}

	if idx := strings.Index(key, "="); idx != -1 {
		ts.Value = key[idx+1:]
		ts.HasValue = true
		key = key[:idx]
	}

	if key == "" {
		return TaintSelector{}, fmt.Errorf("empty key in taint selector %q", s)
	}
	ts.Key = key

	return ts, nil
}

func parseTaintSelectors(exprs []string) ([]TaintSelector, error) {
	selectors := make([]TaintSelector, 0, len(exprs))
	for _, expr := range exprs {
		ts, err := parseTaintSelector(expr)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, ts)
	}
	return selectors, nil
}

// parseLabelSelectors parses each string as a Kubernetes label selector.
func parseLabelSelectors(exprs []string) ([]labels.Selector, error) {
	selectors := make([]labels.Selector, 0, len(exprs))
	for _, expr := range exprs {
		sel, err := labels.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector %q: %w", expr, err)
		}
		selectors = append(selectors, sel)
	}
	return selectors, nil
}
