package hashutil_test

import (
	"encoding/hex"
	"testing"

	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lukechampine.com/blake3"
)

func TestHashBytes_SHA256(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			data:     []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "longer text",
			data:     []byte("The quick brown fox jumps over the lazy dog"),
			expected: "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592",
		},
		{
			name:     "binary data",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe, 0xfd, 0xfc},
			expected: "fed271e1776a1c254c9e8ea187937d24418e1d01781eee828507725de159dd58",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hashutil.HashBytes(tt.data, hashutil.HashAlgoSHA256)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHashBytes_BLAKE3(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "simple string",
			data: []byte("hello world"),
		},
		{
			name: "longer text",
			data: []byte("The quick brown fox jumps over the lazy dog"),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe, 0xfd, 0xfc},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hashutil.HashBytes(tt.data, hashutil.HashAlgoBLAKE3)
			require.NoError(t, err)

			// Compute expected value using blake3 directly
			expectedHash := blake3.Sum256(tt.data)
			expected := hex.EncodeToString(expectedHash[:])

			assert.Equal(t, expected, result)
		})
	}
}

func TestHashBytes_UnsupportedAlgorithm(t *testing.T) {
	result, err := hashutil.HashBytes([]byte("test data"), "unsupported")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported hash algorithm")
	assert.Empty(t, result)
}

func TestHashBytes_Deterministic(t *testing.T) {
	data := []byte("deterministic test data")

	// Run multiple times and verify same result for SHA256
	hash1, err1 := hashutil.HashBytes(data, hashutil.HashAlgoSHA256)
	hash2, err2 := hashutil.HashBytes(data, hashutil.HashAlgoSHA256)
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, hash1, hash2)

	// Run multiple times and verify same result for BLAKE3
	hash3, err3 := hashutil.HashBytes(data, hashutil.HashAlgoBLAKE3)
	hash4, err4 := hashutil.HashBytes(data, hashutil.HashAlgoBLAKE3)
	require.NoError(t, err3)
	require.NoError(t, err4)
	assert.Equal(t, hash3, hash4)
}

func TestHashBytes_DifferentDataProducesDifferentHashes(t *testing.T) {
	data1 := []byte("data set 1")
	data2 := []byte("data set 2")

	// SHA256
	hash1, _ := hashutil.HashBytes(data1, hashutil.HashAlgoSHA256)
	hash2, _ := hashutil.HashBytes(data2, hashutil.HashAlgoSHA256)
	assert.NotEqual(t, hash1, hash2)

	// BLAKE3
	hash3, _ := hashutil.HashBytes(data1, hashutil.HashAlgoBLAKE3)
	hash4, _ := hashutil.HashBytes(data2, hashutil.HashAlgoBLAKE3)
	assert.NotEqual(t, hash3, hash4)
}

func TestHashBytes_LargeData(t *testing.T) {
	// Create a large byte slice (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// SHA256 should handle large data
	hash1, err1 := hashutil.HashBytes(largeData, hashutil.HashAlgoSHA256)
	require.NoError(t, err1)
	assert.Len(t, hash1, 64) // SHA256 produces 32 bytes = 64 hex characters

	// BLAKE3 should handle large data
	hash2, err2 := hashutil.HashBytes(largeData, hashutil.HashAlgoBLAKE3)
	require.NoError(t, err2)
	assert.Len(t, hash2, 64) // BLAKE3 produces 32 bytes = 64 hex characters
}

func TestHashBytes_OutputLength(t *testing.T) {
	data := []byte("test")

	// SHA256 output should be 64 hex characters (32 bytes)
	hash256, _ := hashutil.HashBytes(data, hashutil.HashAlgoSHA256)
	assert.Len(t, hash256, 64)

	// BLAKE3 output should be 64 hex characters (32 bytes)
	hashBlake3, _ := hashutil.HashBytes(data, hashutil.HashAlgoBLAKE3)
	assert.Len(t, hashBlake3, 64)
}

func TestHashBytes_KnownVectors_SHA256(t *testing.T) {
	// SHA256 known test vectors
	vectors := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			input:    "abc",
			expected: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			input:    "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			expected: "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
		},
	}

	for _, v := range vectors {
		result, err := hashutil.HashBytes([]byte(v.input), hashutil.HashAlgoSHA256)
		require.NoError(t, err)
		assert.Equal(t, v.expected, result, "SHA256 hash mismatch for input: %q", v.input)
	}
}

func TestHashBytes_KnownVectors_BLAKE3(t *testing.T) {
	// BLAKE3 known test vectors from the official specification
	vectors := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262",
		},
		{
			input:    "abc",
			expected: "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85",
		},
	}

	for _, v := range vectors {
		result, err := hashutil.HashBytes([]byte(v.input), hashutil.HashAlgoBLAKE3)
		require.NoError(t, err)
		assert.Equal(t, v.expected, result, "BLAKE3 hash mismatch for input: %q", v.input)
	}
}

func TestHashAlgo_Constants(t *testing.T) {
	assert.Equal(t, string(hashutil.HashAlgoSHA256), "sha256")
	assert.Equal(t, string(hashutil.HashAlgoBLAKE3), "blake3")
}
