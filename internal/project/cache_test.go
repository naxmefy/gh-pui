package project

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if _, err := Load("x", 1); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty cache: got %v, want ErrNotExist", err)
	}

	n := 3.0
	p := &Project{Owner: "x", Number: 1, Title: "t", Items: []Item{{ID: "I1", Values: map[string]Value{"Estimate": {Text: "3", Number: &n}}}}}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load("x", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, p)
	}
}
