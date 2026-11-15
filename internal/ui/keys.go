package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
)

// KeyMap grows with the views; every action can be rebound under `keys:` in config.yml.
type KeyMap struct {
	Quit key.Binding
}

func NewKeyMap(overrides map[string][]string) (KeyMap, error) {
	km := KeyMap{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
	bindings := map[string]*key.Binding{"quit": &km.Quit}
	for action, keys := range overrides {
		b, ok := bindings[action]
		if !ok {
			return KeyMap{}, fmt.Errorf("keys: unknown action %q", action)
		}
		b.SetKeys(keys...)
		b.SetHelp(keys[0], b.Help().Desc)
	}
	return km, nil
}
