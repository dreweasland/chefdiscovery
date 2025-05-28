package chef

import (
	"reflect"
	"testing"
)

func TestUnwrapArrayStringSlice(t *testing.T) {
	input := []string{"a", "b", "c"}
	got := unwrapArray(input)
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unwrapArray(%v)=%v, want %v", input, got, want)
	}
}

func TestUnwrapArrayMixedInterfaceSlice(t *testing.T) {
	input := []interface{}{"a", 1, "b", true}
	got := unwrapArray(input)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unwrapArray(%v)=%v, want %v", input, got, want)
	}
}

func TestUnwrapArrayTypedSliceAndNil(t *testing.T) {
	intSlice := []int{1, 2, 3}
	got := unwrapArray(intSlice)
	if len(got) != 0 {
		t.Errorf("unwrapArray(%v)=%v, want empty slice", intSlice, got)
	}

	var nilSlice interface{}
	got = unwrapArray(nilSlice)
	if len(got) != 0 {
		t.Errorf("unwrapArray(nil)=%v, want empty slice", got)
	}
}

func TestDeepMerge(t *testing.T) {
	m1 := map[string]interface{}{"a": 1, "b": map[string]interface{}{"x": 1}}
	m2 := map[string]interface{}{"b": map[string]interface{}{"y": 2}, "c": 3}
	want := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{"x": 1, "y": 2},
		"c": 3,
	}
	got := deepMerge(m1, m2)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deepMerge()=%v, want %v", got, want)
	}

	m3 := map[string]interface{}{"a": 2}
	want["a"] = 2
	got = deepMerge(m1, m2, m3)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deepMerge override=%v, want %v", got, want)
	}
}

func TestMetaAttr(t *testing.T) {
	attrs := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2":  "value",
			"foo_bar": map[string]interface{}{"baz": "found"},
		},
	}
	vm := virtualMachine{Attribute: attrs}

	got := metaAttr(map[string]interface{}{"level1_level2": nil}, vm)
	if got != "value" {
		t.Errorf("metaAttr simple=%v, want value", got)
	}

	got = metaAttr(map[string]interface{}{"level1_foo\\_bar_baz": nil}, vm)
	if got != "found" {
		t.Errorf("metaAttr escaped=%v, want found", got)
	}

	got = metaAttr(map[string]interface{}{"missing": nil}, vm)
	if got != nil {
		t.Errorf("metaAttr missing=%v, want nil", got)
	}
}
