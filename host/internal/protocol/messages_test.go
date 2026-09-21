package protocol

import "testing"

func TestInputMessageValid(t *testing.T) {
	valid := []string{"key", "mouse_move", "mouse_button", "mouse_wheel", "gamepad", "release_all"}
	for _, typ := range valid {
		if !((InputMessage{Type: typ}).Valid()) {
			t.Fatalf("expected %q to be valid", typ)
		}
	}

	if (InputMessage{Type: "shell"}).Valid() {
		t.Fatal("unknown input type must be refused")
	}
}
