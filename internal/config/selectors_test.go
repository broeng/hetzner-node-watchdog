package config

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestParseTaintSelector(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    TaintSelector
		wantErr bool
	}{
		{
			name: "key only",
			in:   "node.cloudprovider.kubernetes.io/shutdown",
			want: TaintSelector{Key: "node.cloudprovider.kubernetes.io/shutdown"},
		},
		{
			name: "key=value",
			in:   "foo=bar",
			want: TaintSelector{Key: "foo", Value: "bar", HasValue: true},
		},
		{
			name: "key:effect",
			in:   "foo:NoSchedule",
			want: TaintSelector{Key: "foo", Effect: corev1.TaintEffectNoSchedule, HasEffect: true},
		},
		{
			name: "key=value:effect",
			in:   "foo=bar:NoExecute",
			want: TaintSelector{Key: "foo", Value: "bar", HasValue: true, Effect: corev1.TaintEffectNoExecute, HasEffect: true},
		},
		{
			name: "value containing a colon-like separator is not mistaken for effect",
			in:   "foo=bar:baz:NoSchedule",
			want: TaintSelector{Key: "foo", Value: "bar:baz", HasValue: true, Effect: corev1.TaintEffectNoSchedule, HasEffect: true},
		},
		{
			name:    "empty string",
			in:      "",
			wantErr: true,
		},
		{
			name:    "empty key with value",
			in:      "=badvalue",
			wantErr: true,
		},
		{
			name:    "empty key with effect",
			in:      ":NoSchedule",
			wantErr: true,
		},
		{
			name:    "empty effect (trailing colon)",
			in:      "foo:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTaintSelector(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTaintSelector(%q): expected error, got none", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTaintSelector(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseTaintSelector(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseTaintSelectors(t *testing.T) {
	t.Run("empty input yields empty, non-nil slice", func(t *testing.T) {
		got, err := parseTaintSelectors(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %#v, want empty non-nil slice", got)
		}
	})

	t.Run("multiple valid entries", func(t *testing.T) {
		got, err := parseTaintSelectors([]string{"foo", "bar=baz:NoSchedule"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d selectors, want 2", len(got))
		}
	})

	t.Run("one invalid entry fails the whole batch", func(t *testing.T) {
		_, err := parseTaintSelectors([]string{"foo", "=bad"})
		if err == nil {
			t.Fatal("expected error, got none")
		}
	})
}

func TestTaintSelectorMatches(t *testing.T) {
	tests := []struct {
		name  string
		sel   TaintSelector
		taint corev1.Taint
		want  bool
	}{
		{
			name:  "key-only selector matches any value/effect",
			sel:   TaintSelector{Key: "foo"},
			taint: corev1.Taint{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoExecute},
			want:  true,
		},
		{
			name:  "key mismatch never matches",
			sel:   TaintSelector{Key: "foo"},
			taint: corev1.Taint{Key: "other"},
			want:  false,
		},
		{
			name:  "value required and matches",
			sel:   TaintSelector{Key: "foo", Value: "bar", HasValue: true},
			taint: corev1.Taint{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule},
			want:  true,
		},
		{
			name:  "value required but mismatches",
			sel:   TaintSelector{Key: "foo", Value: "bar", HasValue: true},
			taint: corev1.Taint{Key: "foo", Value: "other"},
			want:  false,
		},
		{
			name:  "effect required and matches",
			sel:   TaintSelector{Key: "foo", Effect: corev1.TaintEffectNoExecute, HasEffect: true},
			taint: corev1.Taint{Key: "foo", Value: "anything", Effect: corev1.TaintEffectNoExecute},
			want:  true,
		},
		{
			name:  "effect required but mismatches",
			sel:   TaintSelector{Key: "foo", Effect: corev1.TaintEffectNoExecute, HasEffect: true},
			taint: corev1.Taint{Key: "foo", Effect: corev1.TaintEffectNoSchedule},
			want:  false,
		},
		{
			name:  "value and effect both required and both match",
			sel:   TaintSelector{Key: "foo", Value: "bar", HasValue: true, Effect: corev1.TaintEffectNoSchedule, HasEffect: true},
			taint: corev1.Taint{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule},
			want:  true,
		},
		{
			name:  "value matches but effect doesn't",
			sel:   TaintSelector{Key: "foo", Value: "bar", HasValue: true, Effect: corev1.TaintEffectNoSchedule, HasEffect: true},
			taint: corev1.Taint{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoExecute},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sel.Matches(tt.taint); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLabelSelectors(t *testing.T) {
	t.Run("empty input yields empty, non-nil slice", func(t *testing.T) {
		got, err := parseLabelSelectors(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %#v, want empty non-nil slice", got)
		}
	})

	t.Run("valid selector matches expected label sets", func(t *testing.T) {
		got, err := parseLabelSelectors([]string{"role=spot"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d selectors, want 1", len(got))
		}
		if !got[0].Matches(labels.Set{"role": "spot"}) {
			t.Error("expected selector to match role=spot")
		}
		if got[0].Matches(labels.Set{"role": "ondemand"}) {
			t.Error("expected selector not to match role=ondemand")
		}
	})

	t.Run("set-based selector syntax is supported", func(t *testing.T) {
		got, err := parseLabelSelectors([]string{"role in (spot,ondemand)"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got[0].Matches(labels.Set{"role": "spot"}) {
			t.Error("expected selector to match role=spot")
		}
		if !got[0].Matches(labels.Set{"role": "ondemand"}) {
			t.Error("expected selector to match role=ondemand")
		}
		if got[0].Matches(labels.Set{"role": "other"}) {
			t.Error("expected selector not to match role=other")
		}
	})

	t.Run("multiple entries preserved independently", func(t *testing.T) {
		got, err := parseLabelSelectors([]string{"role=spot", "env=staging"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d selectors, want 2", len(got))
		}
	})

	t.Run("invalid selector syntax errors", func(t *testing.T) {
		_, err := parseLabelSelectors([]string{"???not a selector???"})
		if err == nil {
			t.Fatal("expected error, got none")
		}
	})
}

// sanity check that Configuration's field type lines up with what nodeIsIgnored-style
// callers expect: a plain metav1.ObjectMeta-backed node's labels satisfy labels.Set.
func TestParsedSelectorAgainstObjectMeta(t *testing.T) {
	sels, err := parseLabelSelectors([]string{"env=prod"})
	if err != nil {
		t.Fatal(err)
	}
	meta := metav1.ObjectMeta{Labels: map[string]string{"env": "prod"}}
	if !sels[0].Matches(labels.Set(meta.Labels)) {
		t.Error("expected selector to match node labels via labels.Set conversion")
	}
}
