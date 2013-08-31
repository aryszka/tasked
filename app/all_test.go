package app

func eqbytes(bs0, bs1 []byte) bool {
	if len(bs0) != len(bs1) {
		return false
	}
	for i, b := range bs0 {
		if b != bs1[i] {
			return false
		}
	}
	return true
}
