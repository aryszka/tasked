package htproc

import (
	. "code.google.com/p/tasked/share"
	"log"
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
	hostname  string
	portFrom  int
	portTo    int
	maxProcs  int
	procStore *procStore
}

func New(s Settings) *ProcFilter {
	// validate settings
	f := new(ProcFilter)
	f.procStore = newProcStore(s)
	return f
}

func (f *ProcFilter) Run(procErrors chan error) error {
	return f.procStore.run(procErrors)
}

func (f *ProcFilter) Filter(w http.ResponseWriter, r *http.Request, d interface{}) (interface{}, bool) {
	u, _ := d.(string)
	if u == "" {
		log.Println("no user")
		return nil, false
	}
	log.Println("got user")
	for {
		p, err := f.procStore.get(u)
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

func (f *ProcFilter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusNotFound)
}

func (f *ProcFilter) Close() { f.procStore.close() }
