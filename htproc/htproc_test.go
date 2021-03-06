package htproc

import (
	"bytes"
	. "github.com/aryszka/tasked/testing"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"
)

var testLong = false

func init() {
	tl := flag.Bool("test.long", false, "")
	flag.Parse()
	testLong = *tl
}

type testOptions struct {
	maxProcesses int
	dialTimeout  time.Duration
	idleTimeout  int
	cachedir     string
}

func (o *testOptions) MaxUserProcesses() int      { return o.maxProcesses }
func (o *testOptions) DialTimeout() time.Duration { return o.dialTimeout }
func (o *testOptions) ProcessIdleTime() int       { return o.idleTimeout }
func (o *testOptions) Cachedir() string           { return o.cachedir }

func TestNew(t *testing.T) {
	p := New(&testOptions{})
	if p.procStore == nil {
		t.Fail()
	}
}

func TestFilter(t *testing.T) {
	var (
		f        *ProcFilter
		data     interface{}
		dataBack interface{}
		handled  bool
	)
	defer func(c string, a []string) { command, args = c, a }(command, args)
	command = "testproc"
	args = []string{"serve", path.Join(Testdir, "sockets/user0"), string(startupMessage)}
	Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		dataBack, handled = f.Filter(w, r, data)
		if dataBack != data {
			t.Fail()
		}
	}

	// no user
	f = New(&testOptions{})
	data = nil
	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK || handled {
			t.Fail()
		}
	})

	// procStoreClosed
	f = New(&testOptions{})
	close(f.procStore.exit)
	data = "user0"
	Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound || !handled {
			t.Fail()
		}
	})

	// banned
	f = New(&testOptions{})
	f.procStore.banned["user0"] = time.Now()
	data = "user0"
	WithTimeout(t, eto, func() {
		w := Wait(func() { f.procStore.run(nil) })
		Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusNotFound || !handled {
				t.Fail()
			}
		})
		close(f.procStore.exit)
		<-w
	})

	// mirror teapot
	f = New(&testOptions{cachedir: Testdir})
	data = "user0"
	WithTimeout(t, exitTimeout, func() {
		w := Wait(func() { f.procStore.run(nil) })
		body := []byte("hello")
		r, err := http.NewRequest("GET",
			S.URL+fmt.Sprintf("/%d", http.StatusTeapot),
			bytes.NewBuffer(body))
		r.Header.Set("X-Test-Header", "test-header-value")
		ErrFatal(t, err)
		Htreqr(t, r, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusTeapot ||
				rsp.Header.Get("X-Test-Header") != "test-header-value" {
				t.Fail()
			}
			bodyBack, err := ioutil.ReadAll(rsp.Body)
			if err != nil || !bytes.Equal(bodyBack, body) {
				t.Fail()
			}
		})
		close(f.procStore.exit)
		<-w
	})

	// can't connect
	f = New(&testOptions{})
	data = "user0"
	args = []string{"noop"}
	WithTimeout(t, exitTimeout, func() {
		w := Wait(func() { f.procStore.run(nil) })
		Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusNotFound || !handled {
				t.Fail()
			}
		})
		close(f.procStore.exit)
		<-w
	})

	// proc closed
	f = New(&testOptions{})
	data = "user0"
	args = []string{"wait", "4500"}
	WithTimeout(t, exitTimeout, func() {
		w0 := Wait(func() { f.procStore.run(nil) })
		w1 := Wait(func() { Htreq(t, "GET", S.URL, nil, func(rsp *http.Response) {}) })
		time.Sleep(120 * time.Millisecond)
		p0 := f.procStore.procs["user0"]
		f.procStore.removeProc("user0")
		time.Sleep(120 * time.Millisecond)
		p1 := f.procStore.procs["user0"]
		if p0 == p1 || p1 == nil {
			t.Fail()
		}
		f.procStore.close()
		<-w0
		<-w1
	})
}

func TestServeHTTP(t *testing.T) {
	p := New(&testOptions{})
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
