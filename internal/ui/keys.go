package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
)

// KeyMap grows with the views; every action can be rebound under `keys:` in config.yml.
type KeyMap struct {
	Quit     key.Binding
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Toggle   key.Binding
	NextView key.Binding
	PrevView key.Binding
	Switch   key.Binding
	Left     key.Binding
	Right    key.Binding
	Edit     key.Binding
}

func NewKeyMap(overrides map[string][]string) (KeyMap, error) {
	km := KeyMap{
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		Toggle:   key.NewBinding(key.WithKeys("enter", "space"), key.WithHelp("enter", "fold group")),
		NextView: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
		PrevView: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev view")),
		Switch:   key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "projects")),
		Left:     key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "left")),
		Right:    key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "right")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	}
	bindings := map[string]*key.Binding{
		"quit": &km.Quit, "up": &km.Up, "down": &km.Down, "top": &km.Top, "bottom": &km.Bottom,
		"toggle": &km.Toggle, "next_view": &km.NextView, "prev_view": &km.PrevView, "switch_project": &km.Switch,
		"left": &km.Left, "right": &km.Right, "edit": &km.Edit,
	}
	for action, keys := range overrides {
		b, ok := bindings[action]
		if !ok {
			return KeyMap{}, fmt.Errorf("keys: unknown action %q", action)
		}
		if len(keys) == 0 {
			return KeyMap{}, fmt.Errorf("keys: %s needs at least one key", action)
		}
		b.SetKeys(keys...)
		b.SetHelp(keys[0], b.Help().Desc)
	}
	return km, nil
}

func (k KeyMap) help() []key.Binding {
	return []key.Binding{k.Down, k.Up, k.Edit, k.Toggle, k.NextView, k.Switch, k.Quit}
}
