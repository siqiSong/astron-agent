package cellref

import "testing"

func TestParseRange(t *testing.T) {
	got, err := ParseRange("B2:D9")
	if err != nil {
		t.Fatal(err)
	}
	if got.Start.Row != 1 || got.Start.Col != 1 || got.End.Row != 8 || got.End.Col != 3 {
		t.Fatalf("got = %+v", got)
	}
	if got.String() != "B2:D9" {
		t.Fatalf("String() = %q", got.String())
	}
}

func TestParseRangeAcceptsSingleCellAndWhitespace(t *testing.T) {
	got, err := ParseRange("  aa10 ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Start != got.End || got.String() != "AA10" {
		t.Fatalf("got = %+v string=%q", got, got.String())
	}
}

func TestParseRangeRejectsInvalidOrReversedRanges(t *testing.T) {
	for _, input := range []string{"", "A0", "0A", "B2:A1", "A1:B", "A1:B2:Z3", "AMK1"} {
		if _, err := ParseRange(input); err == nil {
			t.Errorf("ParseRange(%q) succeeded", input)
		}
	}
}
