package discord

import "testing"

func TestSplitMessage(t *testing.T) {
	chunks := splitMessage(string(make([]byte, 0))+"你好世界", 3)
	if len(chunks) != 2 || chunks[0] != "你好世" || chunks[1] != "界" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "a\n"
	}
	chunks = splitMessage(long, 20)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > 20 {
			t.Fatalf("chunk too long: %d", len([]rune(c)))
		}
	}
}
