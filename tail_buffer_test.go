package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTailBuffer_SmallWrite(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(100)

	tb.Write([]byte("hello"))
	assert.Equal(t, "hello", tb.String())
}

func TestTailBuffer_ExactSize(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(5)

	tb.Write([]byte("12345"))
	assert.Equal(t, "12345", tb.String())
}

func TestTailBuffer_Overflow(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(10)

	tb.Write([]byte("hello world!!!")) // 14 bytes, buffer is 10
	assert.Equal(t, "ld!!!", tb.String()[5:])
	assert.Equal(t, 10, len(tb.String()))
}

func TestTailBuffer_MultipleWrites(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(10)

	tb.Write([]byte("12345"))
	tb.Write([]byte("67890"))
	assert.Equal(t, "1234567890", tb.String())
}

func TestTailBuffer_MultipleWritesOverflow(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(10)

	tb.Write([]byte("12345"))
	tb.Write([]byte("6789012345"))
	// Should keep last 10 bytes: "6789012345" — wait, total is 15 bytes, keep last 10
	assert.Equal(t, "6789012345", tb.String())
}

func TestTailBuffer_SingleLargeWrite(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(5)

	tb.Write([]byte("abcdefghij")) // 10 bytes, buffer is 5
	assert.Equal(t, "fghij", tb.String())
}

func TestTailBuffer_WrapAround(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(8)

	tb.Write([]byte("12345")) // pos=5
	tb.Write([]byte("6789"))  // wraps: "67" fit, "89" wrap to start → buf = [8,9,3,4,5,6,7,_] wait no
	// Actually: buf=[1,2,3,4,5,_,_,_] pos=5
	// Write "6789": space=3, n=4 > space
	//   copy buf[5:] ← "678", copy buf[0:] ← "9"
	//   pos=1, full=true
	// buf = [9,2,3,4,5,6,7,8]
	// String: buf[1:] + buf[:1] = "2345678" + "9" = "23456789"
	// Hmm, that's 8 chars for the last 9 written... let me just check the last 8 of "123456789"
	assert.Equal(t, "23456789", tb.String())
}

func TestTailBuffer_Empty(t *testing.T) {
	t.Parallel()

	var tb tailBuffer
	tb.Init(10)

	assert.Equal(t, "", tb.String())
}
