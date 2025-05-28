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
