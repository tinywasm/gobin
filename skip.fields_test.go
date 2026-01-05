package binary

import (
	"testing"
)

type skipStruct struct {
	Public      string
	unexported  string
	SkippedJson string `json:"-"`
	SkippedBin  string `binary:"-"`
}

func TestFieldSkipping(t *testing.T) {
	v := &skipStruct{
		Public:      "visible",
		unexported:  "hidden",
		SkippedJson: "should-skip-json",
		SkippedBin:  "should-skip-bin",
	}

	var b []byte
	err := Encode(v, &b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// The encoded data should ONLY contain "visible"
	// If 1.75 etc was encoded in simple_test, we know string is just bytes with length prefix.

	s := &skipStruct{}
	err = Decode(b, s)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if s.Public != "visible" {
		t.Errorf("Expected Public='visible', got %q", s.Public)
	}

	// These should be empty because they should have been skipped during encoding or decoding
	if s.unexported != "" {
		t.Errorf("unexported field should be empty, got %q", s.unexported)
	}
	if s.SkippedJson != "" {
		t.Errorf("SkippedJson field should be empty, got %q", s.SkippedJson)
	}
	if s.SkippedBin != "" {
		t.Errorf("SkippedBin field should be empty, got %q", s.SkippedBin)
	}
}
