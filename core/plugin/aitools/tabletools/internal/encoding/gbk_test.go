package encoding

import "testing"

func TestDecodeGBKChinese(t *testing.T) {
	got, err := DecodeGBK([]byte{0xD0, 0xD5, 0xC3, 0xFB})
	if err != nil {
		t.Fatal(err)
	}
	if got != "姓名" {
		t.Fatalf("got = %q, want 姓名", got)
	}
}

func TestDecodeGBKPreservesASCIIAndSymbols(t *testing.T) {
	got, err := DecodeGBK([]byte{'A', ',', 0xA3, 0xB1})
	if err != nil {
		t.Fatal(err)
	}
	if got != "A,１" {
		t.Fatalf("got = %q", got)
	}
}

func TestDecodeGBKRejectsInvalidSequences(t *testing.T) {
	for _, input := range [][]byte{{0xD0}, {0x80, 0x40}, {0x81, 0x7F}} {
		if _, err := DecodeGBK(input); err == nil {
			t.Errorf("DecodeGBK(%x) succeeded", input)
		}
	}
}
