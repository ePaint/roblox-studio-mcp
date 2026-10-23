package mcp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStructuredValue_WrapsNonObjects(t *testing.T) {
	cases := map[string]any{
		`{"a":1}`: map[string]any{"a": float64(1)},
		`"hello"`: map[string]any{"value": "hello"},
		`42`:      map[string]any{"value": float64(42)},
		`[1,2]`:   map[string]any{"value": []any{float64(1), float64(2)}},
		`null`:    map[string]any{"value": nil},
		``:        map[string]any{"value": nil},
	}

	for raw, want := range cases {
		got := structuredValue(json.RawMessage(raw))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("structuredValue(%q) = %#v, want %#v", raw, got, want)
		}
	}
}

func TestRoleForContext(t *testing.T) {
	cases := map[string]string{"": "edit", "edit": "edit", "server": "server"}
	for context, want := range cases {
		got, ok := roleForContext(context)
		if !ok || got != want {
			t.Errorf("roleForContext(%q) = %q, %v; want %q", context, got, ok, want)
		}
	}

	if _, ok := roleForContext("client"); ok {
		t.Error("expected client context to be rejected")
	}
}
