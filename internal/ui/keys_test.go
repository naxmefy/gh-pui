package ui

import "testing"

func TestKeyMapOverrides(t *testing.T) {
	km, err := NewKeyMap(map[string][]string{"quit": {"x", "ctrl+q"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := km.Quit.Keys(); len(got) != 2 || got[0] != "x" || km.Quit.Help().Key != "x" {
		t.Errorf("quit keys = %v, help %q", got, km.Quit.Help().Key)
	}
	if _, err := NewKeyMap(map[string][]string{"fly": {"f"}}); err == nil {
		t.Error("unknown action should fail")
	}
}
