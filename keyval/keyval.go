package keyval

import (
	"bytes"
	. "code.google.com/p/tasked/share"
	"io"
	"os"
	"unicode"
)

type Entry struct {
	Key string
	Val string
}

type entries []*Entry

func parseFile(r io.RuneReader) (entries, error) {
	type escState int
	const noEsc escState = 0
	const (
		escOne escState = 1 << iota
		escSingle
		escDouble
	)
	type tokenState int
	const (
		undefined tokenState = iota
		key
		value
		comment
	)
	var (
		ts     tokenState
		escn   escState
		es     entries
		e      *Entry
		token  = bytes.NewBuffer(nil)
		spaces = bytes.NewBuffer(nil)
	)
	flush := func() string {
		b := token.String()
		token = bytes.NewBuffer(nil)
		spaces = bytes.NewBuffer(nil)
		return b
	}
	next := func() {
		e = new(Entry)
		es = append(es, e)
		ts = key
	}
	write := func(c rune) error {
		if ts == undefined {
			next()
		}
		if token.Len() > 0 {
			if _, err := io.Copy(token, spaces); err != nil {
				return err
			}
		}
		spaces = bytes.NewBuffer(nil)
		_, err := token.WriteRune(c)
		return err
	}
	for {
		esc := escn
		escn &^= escOne
		c, s, err := r.ReadRune()
		switch {
		case s == 0 || err != nil:
			if err == io.EOF {
				err = nil
			}
			if token.Len() > 0 {
				e.Val = flush()
			}
			return es, err
		case c == rune('\\'):
			switch {
			case esc&escOne == escOne:
				if err := write(c); err != nil {
					return nil, err
				}
			default:
				escn |= escOne
			}
		case c == rune('\''):
			switch {
			case ts == comment:
			case esc&^escSingle != noEsc:
				if err := write(c); err != nil {
					return nil, err
				}
			case esc == escSingle:
				escn = noEsc
			default:
				escn = escSingle
			}
		case c == rune('"'):
			switch {
			case ts == comment:
			case esc&^escDouble != noEsc:
				if err := write(c); err != nil {
					return nil, err
				}
			case esc == escDouble:
				escn = noEsc
			default:
				escn = escDouble
			}
		case c == rune('='):
			switch {
			case ts == comment:
			case esc != noEsc || ts == value:
				if err := write(c); err != nil {
					return nil, err
				}
			default:
				if ts == undefined {
					next()
				}
				e.Key = string(flush())
				ts = value
			}
		case c == rune('\n'):
			switch {
			case esc != noEsc && ts == comment:
			case esc != noEsc && ts != comment:
				if err := write(c); err != nil {
					return nil, err
				}
			case esc == noEsc && ts != undefined:
				if token.Len() > 0 {
					e.Val = flush()
				}
				ts = undefined
			}
		case c == rune('#'):
			switch {
			case ts == comment:
			case esc != noEsc:
				if err := write(c); err != nil {
					return nil, err
				}
			default:
				ts = comment
			}
		default:
			switch {
			case ts == comment:
			case esc == noEsc && unicode.IsSpace(c):
				if _, err := spaces.WriteRune(c); err != nil {
					return nil, err
				}
			default:
				if err := write(c); err != nil {
					return nil, err
				}
			}
		}
	}
}

func readFile(fn string) (entries, error) {
	f, err := os.Open(fn)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return nil, err
	}
	defer Doretlog42(f.Close)
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	return parseFile(buf)
}

func expandIncludes(e entries, includeKeys []string, isRead map[string]bool) (entries, error) {
	if len(includeKeys) == 0 {
		return e, nil
	}
	for i := len(e) - 1; i >= 0; i-- {
		Entry := e[i]
		isInclude := false
		for _, key := range includeKeys {
			isInclude = Entry.Key == key
			if isInclude {
				break
			}
		}
		if !isInclude {
			continue
		}
		fn := string(Entry.Val)
		if isRead[fn] {
			e = append(e[:i], e[i+1:]...)
			continue
		}
		include, err := readFile(fn)
		if err != nil {
			return nil, err
		}
		isRead[fn] = true
		include, err = expandIncludes(include, includeKeys, isRead)
		if err != nil {
			return nil, err
		}
		e = append(e[:i], append(include, e[i+1:]...)...)
	}
	return e, nil
}

func ParseFile(f string) ([]*Entry, error) {
	e, err := readFile(f)
	return []*Entry(e), err
}

func Parse(files []string, includeKeys ...string) ([]*Entry, error) {
	var e entries
	for _, f := range files {
		ei, err := readFile(f)
		if err != nil {
			return nil, err
		}
		e = append(e, ei...)
	}
	e, err := expandIncludes(e, includeKeys, make(map[string]bool))
	return []*Entry(e), err
}
