package doc

import "strings"

// SearchVocab is the word list the search's large documents draw from:
// filler words that never contain the query word.
const SearchVocab = "document rendering parser markdown section viewer window text layout block inline source reader theme scrolling keyboard sidebar"

// NeedleWords returns a Markdown document of about words words of filler
// from SearchVocab and the query word "needle" every nth word, nowhere
// else; it returns the word count the document holds.
func NeedleWords(words, nth int) (string, int) {
	vocab := strings.Fields(SearchVocab)
	var sb strings.Builder
	sb.Grow(words * 10)
	needles := 0
	for i := 0; i < words; i++ {
		if i%nth == nth-1 {
			sb.WriteString("needle ")
			needles++
		} else {
			sb.WriteString(vocab[i%len(vocab)])
			sb.WriteByte(' ')
		}
		if i%40 == 39 {
			sb.WriteByte('\n')
		}
		if i%360 == 359 {
			sb.WriteByte('\n')
		}
	}
	return sb.String(), needles
}
