//go:build test

package service

// Testing wordlist that will always print `a a a a`.
var Wordlist = []string{"a", "a", "a", "a"}

// PassphraseWordCount is the number of words in a passphrase.
const PassphraseWordCount uint8 = 4
