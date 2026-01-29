package protocol

import "crypto/rand"

// Copied from crypto/rand.
// TODO: once 1.24 is assured, just use crypto/rand.
const base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func RandText() string {
	// ⌈log₃₂ 2¹²⁸⌉ = 26 chars
	src := make([]byte, 26)
	rand.Read(src)
	for i := range src {
		src[i] = base32alphabet[src[i]%32]
	}
	return string(src)
}
