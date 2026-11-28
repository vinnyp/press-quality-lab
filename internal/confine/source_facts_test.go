package confine

import "testing"

func TestSourceStreamFacts(t *testing.T) {
	// QH27, QE10: stream facts describe kept bytes; each malformed byte gets its own replacement.
	got := sourceStream([]byte{0xff, 0xff, 'a'}, false)
	if got.Digest != "11c1c47b33eec4162bca33ce7c9a0512d2dfc3ea2146541adc02d988cc6874cc" || got.Head != "\ufffd\ufffda" || !got.Kept {
		t.Fatalf("facts %+v", got)
	}
	cut := sourceStream([]byte("abc"), true)
	if cut.Digest != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || cut.Head != "abc" || cut.Kept {
		t.Fatalf("cut %+v", cut)
	}
	data := make([]byte, 65537)
	for i := range data {
		data[i] = 'a'
	}
	if got := sourceStream(data, false); len(got.Head) != 65536 || got.Digest != "008ffc88d3c96a9f307524eb361e47c5222a887fc45fa0c1fb8d429c5c23b430" {
		t.Fatalf("head length %d", len(got.Head))
	}
}
