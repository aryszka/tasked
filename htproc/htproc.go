package htproc

import (
	. "code.google.com/p/tasked/share"
	"net/http"
	"time"
)

type Settings interface {
	Hostname() string
	PortRange() (int, int)
	MaxProcesses() int
	IdleTimeout() time.Duration
}

type ProcFilter struct {
	hostname string
	portFrom int
	portTo   int
	maxProcs int
	ProcStore       *procStore
}

func New(s Settings) *ProcFilter {
	// validate settings
	f := new(ProcFilter)
	f.ProcStore = newProcStore(s)
	return f
}

func (f *ProcFilter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, h := f.Filter(w, r, nil)
	if !h {
		ErrorResponse(w, http.StatusNotFound)
	}
}

func (f *ProcFilter) Filter(w http.ResponseWriter, r *http.Request, d interface{}) (interface{}, bool) {
	u, _ := d.(string)
	if u == "" {
		return nil, false
	}
	for {
		p, err := f.ProcStore.get(u)
		if !CheckHandle(w, err != procStoreClosed, http.StatusNotFound) ||
			!CheckServerError(w, err == nil) {
			return nil, true
		}
		err = p.serve(w, r)
		if err == nil || !CheckServerError(w, err == procClosed) {
			return nil, true
		}
	}
}

func (f *ProcFilter) Close() { f.ProcStore.close() }
