package main

// tailBuffer is a fixed-size ring buffer that implements io.Writer.
// It retains only the last `size` bytes written to it.
type tailBuffer struct {
	buf  []byte
	size int
	pos  int
	full bool
}

// Init sets the buffer capacity in bytes.
func (tb *tailBuffer) Init(size int) {
	tb.buf = make([]byte, size)
	tb.size = size
}

// Write implements io.Writer. It always succeeds and stores the tail of the data.
func (tb *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n >= tb.size {
		// Data larger than buffer — just keep the last `size` bytes
		copy(tb.buf, p[n-tb.size:])
		tb.pos = 0
		tb.full = true
		return n, nil
	}

	// How much fits before wrapping
	space := tb.size - tb.pos
	if n <= space {
		copy(tb.buf[tb.pos:], p)
		tb.pos += n
	} else {
		copy(tb.buf[tb.pos:], p[:space])
		copy(tb.buf, p[space:])
		tb.pos = n - space
		tb.full = true
	}

	if tb.pos == tb.size {
		tb.pos = 0
		tb.full = true
	}

	return n, nil
}

// String returns the buffered content in order.
func (tb *tailBuffer) String() string {
	if !tb.full {
		return string(tb.buf[:tb.pos])
	}
	// Ring buffer wrapped — concatenate from pos to end, then start to pos
	result := make([]byte, tb.size)
	copy(result, tb.buf[tb.pos:])
	copy(result[tb.size-tb.pos:], tb.buf[:tb.pos])
	return string(result)
}
