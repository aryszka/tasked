package htproc

import (
	tst "code.google.com/p/tasked/testing"
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
	to := make(chan int)
	go func() {
		err := p.Run(nil)
		if err != nil {
			t.Fail()
		}
		to <- 0
	}()
	var (
		data    interface{}
		handled bool
	)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		data, handled = p.Filter(w, r, data)
	}

	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 200 || handled {
			t.Fail()
		}
	})

	data = "user0"
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 200 || !handled {
			t.Fail()
		}
	})

	p.procStore.close()
	data = "user0"
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 404 || !handled {
			t.Fail()
		}
	})
	<-to
}

func TestServeHTTP(t *testing.T) {
	t.Skip()
	p := New(&testSettings{})
	to := make(chan int)
	go func() {
		err := p.Run(nil)
		if err != nil {
			t.Fail()
		}
		to <- 0
	}()
	tst.Thnd.Sh = p.ServeHTTP

	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != 404 {
			t.Fail()
		}
	})
	p.procStore.close()
	<-to
}
