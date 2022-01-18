package usync

import "testing"

func TestHash(t *testing.T) {
	s := "the quick brown fox"

	byteHash := HashBytes([]byte(s))
	stringHash := HashString(s)

	if byteHash != stringHash {
		t.Fatalf("hashes not equal (%x != %x)", byteHash, stringHash)
	}
}

var ( // protect against optimizer
	stringResult uintptr
	bytesResult  uintptr
)

func BenchmarkHashBytes(b *testing.B) {
	buf := []byte("the quick brown fox")

	for i := 0; i < b.N; i++ {
		bytesResult = HashBytes(buf)
	}
}

func BenchmarkHashBytesString(b *testing.B) {
	s := "the quick brown fox"

	for i := 0; i < b.N; i++ {
		bytesResult = HashBytes([]byte(s))
	}
}

func BenchmarkHashString(b *testing.B) {
	s := "the quick brown fox"

	for i := 0; i < b.N; i++ {
		stringResult = HashString(s)
	}
}
