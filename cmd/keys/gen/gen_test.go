package gen

import "testing"

func TestNewCommandDefinesNameFlag(t *testing.T) {
	cmd := NewCommand()

	flag := cmd.Flags().Lookup("name")
	if flag == nil {
		t.Fatal("expected --name flag")
	}
	if flag.Shorthand != "n" {
		t.Fatalf("expected -n shorthand, got %q", flag.Shorthand)
	}
	if cmd.Flags().Lookup("name-override") != nil {
		t.Fatal("did not expect old --name-override flag")
	}
}
