package htproc

import (
	tst "code.google.com/p/tasked/testing"
	"log"
	"net/http"
	"testing"
	"time"
)

type testSettings struct {
	hostname      string
	portRangeFrom int
	portRangeTo   int
	maxProcesses  int
	idleTimeout   time.Duration
}

func TestNew(t *testing.T) {
	p := New(&testSettings{})
	if p.procStore == nil {
		t.Fail()
	}
}

func TestFilter(t *testing.T) {
	p := New(&testSettings{})
	to := make(chan int)
	go func() {
		err := p.Run(nil)
		if err != nil {
			t.Log("proc store failed")
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
	log.Println("started")
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
	p := New(&testSettings{})
	to := make(chan int)
	go func() {
		err := p.Run(nil)
		if err != nil {
			t.Log("proc store failed")
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
