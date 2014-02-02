package htproc

import (
	. "code.google.com/p/tasked/testing"
	"net/http"
	"testing"
	"time"
	"flag"
)

var testLong = false

func init() {
	tl := flag.Bool("test.long", false, "")
	flag.Parse()
	testLong = *tl
}

type testSettings struct {
	hostname      string
	portRangeFrom int
	portRangeTo   int
	maxProcesses  int
	idleTimeout   time.Duration
}

func (s *testSettings) Hostname() string           { return s.hostname }
func (s *testSettings) PortRange() (int, int)      { return s.portRangeFrom, s.portRangeTo }
func (s *testSettings) MaxProcesses() int          { return s.maxProcesses }
func (s *testSettings) IdleTimeout() time.Duration { return s.idleTimeout }

func TestNew(t *testing.T) {
	p := New(&testSettings{})
	if p.procStore == nil {
		t.Fail()
	}
}

func TestFilter(t *testing.T) {
	t.Skip()
	p := New(&testSettings{})
	w := Wait(func() {
		err := p.Run(nil)
		if err != nil {
			t.Fail()
		}
	})
	var (
		data    interface{}
		handled bool
	)
	Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		data, handled = p.Filter(w, r, data)
	}

	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 200 || handled {
			t.Fail()
		}
	})

	data = "user0"
	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 200 || !handled {
			t.Fail()
		}
	})

	p.procStore.close()
	data = "user0"
	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 404 || !handled {
			t.Fail()
		}
	})
	<-w
}

func TestServeHTTP(t *testing.T) {
	t.Skip()
	p := New(&testSettings{})
	w := Wait(func() {
		err := p.Run(nil)
		if err != nil {
			t.Fail()
		}
	})
	Thnd.Sh = p.ServeHTTP

	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 404 {
			t.Fail()
		}
	})
	p.procStore.close()
	<-w
}
