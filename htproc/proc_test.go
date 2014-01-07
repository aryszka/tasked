package htproc

import (
	"testing"
	"time"
	"io"
	"bytes"
)

type testReader struct {
	lines [][]byte
	readIndex int
	err error
	errIndex int
}

type testWriter struct {
	lines [][]byte
	writeIndex int
	err error
	errIndex int
}

var (
	testuser = "testuser"
	eto = 30 * time.Millisecond
)

func (r *testReader) Read(p []byte) (int, error) {
	if len(r.lines) == 0 {
		return 0, io.EOF
	}
	for i := 0; i < len(p); i++ {
		r.readIndex++
		if r.err != nil && r.readIndex == r.errIndex {
			return i, r.err
		}
		if len(r.lines[0]) == 0 {
			p[i] = '\n'
			r.lines = r.lines[1:]
			if len(r.lines) == 0 {
				return i, io.EOF
			}
			continue
		}
		p[i] = r.lines[0][0]
		r.lines[0] = r.lines[0][1:]
	}
	return len(p), nil
}

func (w *testWriter) Write(p []byte) (int, error) {
	if len(w.lines) == 0 {
		w.lines = append(w.lines, nil)
	}
	for i, b := range p {
		if w.err != nil && i == w.errIndex {
			return i, w.err
		}
		if b == '\n' {
			w.lines = append(w.lines, nil)
			continue
		}
		w.lines[len(w.lines) - 1] = append(w.lines[len(w.lines) - 1], b)
	}
	return len(p), nil
}

func linesEqual(l0, l1 [][]byte) bool {
	if len(l0) != len(l1) {
		return false
	}
	for i, l := range l0 {
		if !bytes.Equal(l, l1[i]) {
			return false
		}
	}
	return true
}

func TestNewProc(t *testing.T) {
	t0 := time.Now()
	p := newProc(testuser)
	t1 := time.Now()
	if p.user != testuser {
		t.Fail()
	}
	if p.access.Before(t0) || p.access.After(t1) {
		t.Fail()
	}
}

func TestFilterLines(t *testing.T) {
	one, two, three := []byte("one"), []byte("two"), []byte("three")
	// copy one line
	w := new(testWriter)
	l := one
	lr := filterLines(w, &testReader{lines: [][]byte{l}})
	select {
	case li := <-lr:
		if li.err != io.EOF || len(w.lines) != 1 || !linesEqual(w.lines, [][]byte{l}) {
			t.Fail()
		}
	case <-time.After(eto):
		t.Fail()
	}

	// copy three lines
	w = new(testWriter)
	l0 := one
	l1 := two
	l2 := three
	lr = filterLines(w, &testReader{lines: [][]byte{l0, l1, l2}})
	select {
	case li := <-lr:
		if li.err != io.EOF || len(w.lines) != 3 || !linesEqual(w.lines, [][]byte{l0, l1, l2}) {
			t.Fail()
		}
	case <-time.After(eto):
		t.Fail()
	}

	// find a line
	w = new(testWriter)
	l0 = one
	l1 = two
	l2 = three
	lr = filterLines(w, &testReader{lines: [][]byte{l0, l1, l2}}, two)
	found := false
	loop0: for {
		select {
		case li := <-lr:
			if found {
				if li.err != io.EOF || len(w.lines) != 2 || !linesEqual(w.lines, [][]byte{l0, l2}) {
					t.Fail()
				}
				break loop0
			} else {
				if !bytes.Equal(li.line, two) {
					t.Fail()
				}
				found = true
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// find a line, with delimiter
	w = new(testWriter)
	l0 = one
	l1 = two
	l2 = three
	lr = filterLines(w, &testReader{lines: [][]byte{l0, l1, l2}}, append(two, '\n'))
	found = false
	loop1: for {
		select {
		case li := <-lr:
			if found {
				if li.err != io.EOF || len(w.lines) != 2 || !linesEqual(w.lines, [][]byte{l0, l2}) {
					t.Fail()
				}
				break loop1
			} else {
				if !bytes.Equal(li.line, append(two, '\n')) {
					t.Fail()
				}
				found = true
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// find multiple lines
	w = new(testWriter)
	l0 = one
	l1 = two
	l2 = three
	lr = filterLines(w, &testReader{lines: [][]byte{l0, l1, l2}}, append(two, '\n'), three)
	var rl [][]byte
	loop2: for {
		select {
		case li := <-lr:
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != io.EOF {
					t.Fail()
				}
				break loop2
			}
		case <-time.After(eto):
			t.Fail()
		}
	}
	if len(w.lines) < 1 || len(w.lines) > 2 ||
		!bytes.Equal(w.lines[0], one) || len(w.lines) == 2 && len(w.lines[1]) != 0 ||
		len(rl) < 2 || len(rl) > 3 ||
		!bytes.Equal(rl[0], append(two, '\n')) || !bytes.Equal(rl[1], three) ||
		len(rl) == 3 && len(rl[2]) != 0 {
		t.Fail()
	}

	// detect non-eof read error
}
