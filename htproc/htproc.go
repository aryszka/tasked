package htproc

import (
	. "github.com/aryszka/tasked/share"
	"net/http"
)

type Options interface {
	MaxUserProcesses() int
	ProcessIdleTime() int
	Cachedir() string
}

type ProcFilter struct {
	hostname  string
	portFrom  int
	portTo    int
	maxProcs  int
	procStore *procStore
}

func New(o Options) *ProcFilter {
	// todo: validate options, apply defaults if not set
	f := new(ProcFilter)
	f.procStore = newProcStore(o)
	return f
}

func (f *ProcFilter) Run(procErrors chan error) error {
	return f.procStore.run(procErrors)
}

func (f *ProcFilter) Filter(w http.ResponseWriter, r *http.Request, d interface{}) (interface{}, bool) {
	u, _ := d.(string)
	if u == "" {
		return d, false
	}
	for {
		p, err := f.procStore.getCreate(u)
		if !CheckHandle(w, err != procStoreClosed && err != temporarilyBanned, http.StatusNotFound) ||
			!CheckServerError(w, err == nil) {
			return d, true
		}
		err = p.serve(w, r)
		if err == nil {
			return d, true
		}
		if err != procClosed {
			return d, true
		}
		// todo: diag log non procClosed errors
	}
}

func (f *ProcFilter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusNotFound)
}

func (f *ProcFilter) Close() { f.procStore.close() }
