package lanshare

import "testing"

func TestShareNameValidation(t *testing.T) {
	if !shareNameRE.MatchString("HubaBox") {
		t.Fatal("HubaBox should be valid")
	}
	if shareNameRE.MatchString("") || shareNameRE.MatchString("-bad") {
		t.Fatal("invalid names should not match")
	}
}

func TestUNCPathsFor(t *testing.T) {
	paths := UNCPathsFor("mybox", "HubaBox", []string{"192.168.1.10"})
	if len(paths) < 2 {
		t.Fatalf("want multiple UNC paths, got %v", paths)
	}
	found := false
	for _, p := range paths {
		if p == `\\192.168.1.10\HubaBox` {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing LAN IP UNC: %v", paths)
	}
}
