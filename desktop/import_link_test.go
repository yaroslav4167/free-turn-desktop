package main

import (
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/backend"
)

func TestImportLinkNameAndLocalOverride(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	store, err := backend.NewStore()
	if err != nil {
		t.Fatal(err)
	}
	a := &App{store: store}
	for _, test := range []struct{ linkName, localName, want string }{
		{"Германия 🇩🇪", "", "Германия 🇩🇪"},
		{"Германия 🇩🇪", "Мой сервер", "Мой сервер"},
		{"Германия 🇩🇪", "  ", "Германия 🇩🇪"},
		{"", "Local", "Local"},
		{"", "", ""},
	} {
		raw, err := (backend.ShareLink{Provider: "vk", Peer: "203.0.113.5:56000", Name: test.linkName}).Encode()
		if err != nil {
			t.Fatal(err)
		}
		base := backend.DefaultServer()
		base.Name = test.localName
		saved, err := a.ImportLink(raw, base)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Name != test.want {
			t.Fatalf("link=%q local=%q: got %q, want %q", test.linkName, test.localName, saved.Name, test.want)
		}
		stored, ok := store.Get(saved.ID)
		if !ok || stored.Name != test.want {
			t.Fatalf("name was not persisted: %+v", stored)
		}
	}
}
