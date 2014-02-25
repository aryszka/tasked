package main

import "log"

func main() {
	o, err := readOptions()
	if err != nil {
		log.Panicln(err)
	}
	cmd := o.Command()
	switch cmd {
	case cmdServe:
		s := newServer()
		err := s.serve(o)
		if err != nil {
			log.Panicln(err)
		}
	default:
		log.Panicln("not there yet, not implemented")
	}
}
