package analytics

import (
	"sort"
	"strings"
)

type Analytics struct{}

// commonWords is a map of frequently occurring words that should be ignored in frequency analysis.
// This list can be extended as needed.
var commonWords = map[string]struct{}{
	"a": {}, "about": {}, "above": {}, "across": {}, "after": {}, "afterwards": {},
	"again": {}, "against": {}, "all": {}, "almost": {}, "alone": {}, "along": {},
	"already": {}, "also": {}, "although": {}, "always": {}, "am": {}, "among": {},
	"amongst": {}, "amount": {}, "an": {}, "and": {}, "another": {}, "any": {},
	"anyhow": {}, "anyone": {}, "anything": {}, "anyway": {}, "anywhere": {},
	"are": {}, "aren't": {}, "around": {}, "as": {}, "at": {},

	"back": {}, "be": {}, "became": {}, "because": {}, "become": {}, "becomes": {},
	"becoming": {}, "been": {}, "before": {}, "beforehand": {}, "behind": {},
	"being": {}, "below": {}, "beside": {}, "besides": {}, "between": {},
	"beyond": {}, "both": {}, "but": {}, "by": {},

	"can": {}, "can't": {}, "cannot": {}, "could": {}, "couldn't": {},

	"did": {}, "didn't": {}, "do": {}, "does": {}, "doesn't": {}, "doing": {},
	"don't": {}, "done": {}, "down": {}, "during": {},

	"each": {}, "either": {}, "else": {}, "elsewhere": {}, "enough": {},
	"entirely": {}, "especially": {}, "etc": {}, "even": {}, "ever": {},
	"every": {}, "everyone": {}, "everything": {}, "everywhere": {},

	"few": {}, "for": {}, "former": {}, "formerly": {}, "from": {},
	"further": {},

	"had": {}, "hadn't": {}, "has": {}, "hasn't": {}, "have": {}, "haven't": {},
	"having": {}, "he": {}, "he'd": {}, "he'll": {}, "he's": {}, "hence": {},
	"her": {}, "here": {}, "hereafter": {}, "hereby": {}, "herein": {},
	"here's": {}, "hereupon": {}, "hers": {}, "herself": {}, "him": {},
	"himself": {}, "his": {}, "how": {}, "however": {},

	"i": {}, "i'd": {}, "i'll": {}, "i'm": {}, "i've": {},
	"if": {}, "in": {}, "indeed": {}, "into": {}, "is": {}, "isn't": {},
	"it": {}, "it's": {}, "its": {}, "itself": {},

	"just": {},

	"keep": {},

	"last": {}, "latter": {}, "latterly": {}, "least": {}, "less": {},
	"let": {}, "let's": {}, "like": {}, "likely": {},

	"made": {}, "make": {}, "many": {}, "may": {}, "maybe": {}, "me": {},
	"meanwhile": {}, "might": {}, "mine": {}, "more": {}, "moreover": {},
	"most": {}, "mostly": {}, "much": {}, "must": {}, "mustn't": {},
	"my": {}, "myself": {},

	"neither": {}, "never": {}, "nevertheless": {}, "next": {}, "no": {},
	"nobody": {}, "none": {}, "noone": {}, "nor": {}, "not": {},
	"nothing": {}, "now": {}, "nowhere": {},

	"of": {}, "off": {}, "often": {}, "on": {}, "once": {}, "one": {},
	"only": {}, "onto": {}, "or": {}, "other": {}, "others": {},
	"otherwise": {}, "our": {}, "ours": {}, "ourselves": {}, "out": {},
	"over": {}, "own": {},

	"part": {}, "per": {}, "perhaps": {}, "please": {}, "put": {},

	"rather": {}, "re": {}, "same": {}, "see": {}, "seem": {}, "seemed": {},
	"seeming": {}, "seems": {}, "several": {}, "she": {}, "she'd": {},
	"she'll": {}, "she's": {}, "should": {}, "shouldn't": {}, "since": {},
	"so": {}, "some": {}, "somehow": {}, "someone": {}, "something": {},
	"sometime": {}, "sometimes": {}, "somewhere": {}, "still": {},
	"such": {},

	"take": {}, "than": {}, "that": {}, "that's": {}, "the": {},
	"their": {}, "theirs": {}, "them": {}, "themselves": {}, "then": {},
	"thence": {}, "there": {}, "thereafter": {}, "thereby": {},
	"therefore": {}, "therein": {}, "there's": {}, "thereupon": {},
	"these": {}, "they": {}, "they'd": {}, "they'll": {}, "they're": {},
	"they've": {}, "this": {}, "those": {}, "through": {}, "throughout": {},
	"thru": {}, "thus": {}, "to": {}, "together": {}, "too": {},
	"toward": {}, "towards": {},

	"under": {}, "until": {}, "up": {}, "upon": {}, "us": {}, "use": {},

	"very": {}, "via": {},

	"was": {}, "wasn't": {}, "we": {}, "we'd": {}, "we'll": {},
	"we're": {}, "we've": {}, "well": {}, "were": {}, "weren't": {},
	"what": {}, "whatever": {}, "what's": {}, "when": {}, "whence": {},
	"whenever": {}, "where": {}, "whereafter": {}, "whereas": {},
	"whereby": {}, "wherein": {}, "where's": {}, "whereupon": {},
	"wherever": {}, "whether": {}, "which": {}, "while": {}, "whither": {},
	"who": {}, "who'd": {}, "whoever": {}, "who'll": {}, "who's": {},
	"whose": {}, "why": {}, "with": {}, "within": {}, "without": {},
	"won't": {}, "would": {}, "wouldn't": {},

	"yet": {}, "you": {}, "you'd": {}, "you'll": {}, "you're": {},
	"you've": {}, "your": {}, "yours": {}, "yourself": {}, "yourselves": {},

	// Additional contractions and variants
	"ain't": {}, "it'll": {}, "shan't": {}, "that'll": {}, "when's": {},

	// Common web/UI noise words
	"click": {}, "clickable": {}, "clicked": {}, "clicking": {},
	"button": {}, "link": {}, "menu": {},
	"redirected": {}, "redirect": {}, "redirecting": {},
	"page": {}, "pages": {}, "website": {}, "site": {},
	"home": {}, "homepage": {},
	"search": {}, "searching": {}, "searched": {},
	"loading": {}, "loaded": {}, "load": {}, "loads": {},
}

// IsStopword checks if a word is a common stopword that should be filtered out.
func IsStopword(word string) bool {
	_, exists := commonWords[strings.ToLower(word)]
	return exists
}

func (a *Analytics) WordFrequency(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text)) // strings.Fields handles multiple spaces and newlines
	frequencies := make(map[string]int)

	for _, word := range words {
		// Remove punctuation from words
		word = strings.TrimFunc(word, func(r rune) bool {
			// Keep only lowercase letters and numbers
			return ('a' > r || r > 'z') && ('0' > r || r > '9')
		})

		// Skip if it's a common word or empty after cleaning
		if _, exists := commonWords[word]; exists || word == "" {
			continue
		}

		frequencies[word]++
	}

	return frequencies
}

type wordCount struct {
	Word  string
	Count int
}

func (a *Analytics) TopNWords(text string, n int) []string {
	frequencies := a.WordFrequency(text)

	counts := make([]wordCount, 0, len(frequencies))
	for k, v := range frequencies {
		counts = append(counts, wordCount{k, v})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count > counts[j].Count
	})

	limit := n
	if len(counts) < n {
		limit = len(counts)
	}

	topN := make([]string, limit)
	for i := 0; i < limit; i++ {
		topN[i] = counts[i].Word
	}

	return topN
}
