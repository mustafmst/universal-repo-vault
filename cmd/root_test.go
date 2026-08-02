package cmd

import "testing"

func TestRootCommandIncludesExpectedSubcommands(t *testing.T) {
	root := NewRootCommand()

	want := []string{"init", "encrypt", "decrypt", "keys", "status"}
	for _, name := range want {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if cmd == nil || cmd.Name() != name {
			t.Fatalf("expected root command to include %q", name)
		}
	}
}

func TestKeysCommandIncludesExpectedSubcommands(t *testing.T) {
	root := NewRootCommand()
	keysCmd, _, err := root.Find([]string{"keys"})
	if err != nil {
		t.Fatal(err)
	}
	if keysCmd == nil {
		t.Fatal("expected keys command")
	}

	want := []string{"gen", "add", "list"}
	for _, name := range want {
		cmd, _, err := keysCmd.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if cmd == nil || cmd.Name() != name {
			t.Fatalf("expected keys command to include %q", name)
		}
	}
}
