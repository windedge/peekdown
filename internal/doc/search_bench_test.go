package doc

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// The benchmarks share one benchmark document, built once: about 2000
// top-level blocks of about 2.5 KiB each (headings and fenced code mixed
// in), holding "active" roughly every thousandth word, about 0.1%.

var (
	benchOnce  sync.Once
	benchModel *Model
	benchWords int
)

func benchDoc(b *testing.B) *Model {
	b.Helper()
	benchOnce.Do(func() {
		var sb strings.Builder
		sb.Grow(5 << 20)
		word := 0
		vocab := strings.Fields(SearchVocab)
		writeWord := func() {
			if word%1000 == 999 {
				if (word/1000)%5 == 4 {
					sb.WriteString("Active ")
				} else {
					sb.WriteString("active ")
				}
			} else {
				sb.WriteString(vocab[word%len(vocab)])
				sb.WriteByte(' ')
			}
			word++
		}
		for i := 0; i < 2000; i++ {
			if i%10 == 0 {
				fmt.Fprintf(&sb, "## Section %d\n\n", i)
			}
			if i%50 == 25 {
				fmt.Fprintf(&sb, "```go\nfor i := 0; i < %d; i++ {\n\tprocess(block %d)\n}\n```\n\n", i, i)
			}
			for line := 0; line < 9; line++ {
				for w := 0; w < 40; w++ {
					writeWord()
				}
				sb.WriteByte('\n')
			}
			sb.WriteByte('\n')
		}
		benchModel = New(sb.String())
		benchWords = word
	})
	b.Logf("document: %d words, %d bytes, %d top-level blocks", benchWords, len(benchModel.Source()), benchModel.Len())
	return benchModel
}

func BenchmarkFindAll5MBPlain(b *testing.B) {
	m := benchDoc(b)
	if got := FindAll(m, "active", SearchOptions{}); len(got) < benchWords/1000-1 {
		b.Fatalf("plain: %d matches, want about %d", len(got), benchWords/1000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindAll(m, "active", SearchOptions{})
	}
}

func BenchmarkFindAll5MBRegex(b *testing.B) {
	m := benchDoc(b)
	if got := FindAll(m, `[Aa]ctive[\s]?`, SearchOptions{Regex: true}); len(got) < benchWords/1000-1 {
		b.Fatalf("regex: %d matches, want about %d", len(got), benchWords/1000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindAll(m, `[Aa]ctive[\s]?`, SearchOptions{Regex: true})
	}
}

// BenchmarkFindAllHitLimit queries a letter that hits far past the cap,
// so the scan must stop at the limit instead of running to the end.
func BenchmarkFindAllHitLimit(b *testing.B) {
	m := benchDoc(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := FindAll(m, "e", SearchOptions{})
		if len(got) != defaultLimit {
			b.Fatalf("limit: %d matches, want the cap %d", len(got), defaultLimit)
		}
	}
}
