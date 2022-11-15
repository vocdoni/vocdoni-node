package downloader

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/data"
)

func TestDownloader(t *testing.T) {
	stg := data.DataMockTest{}
	stg.Init(nil)
	d := NewDownloader(&stg)
	d.Start()
	qt.Assert(t, d.QueueSize(), qt.Equals, int32(0))

	callbackChan := make(chan bool)
	callback := func(uri string, data []byte) {
		qt.Assert(t, data, qt.IsNotNil)
		callbackChan <- true
		t.Logf("got file %s\n", uri)
	}

	d.AddToQueue(stg.URIprefix()+"testfile1", callback, false)
	<-callbackChan // wait for retrieving the file
	qt.Assert(t, d.QueueSize(), qt.Equals, int32(0))

	d.AddToQueue(stg.URIprefix()+"testfile2", callback, true)
	d.AddToQueue(stg.URIprefix()+"testfile3", callback, true)
	d.AddToQueue(stg.URIprefix()+"testfile4", callback, true)
	time.Sleep(100 * time.Millisecond)
	qt.Assert(t, d.QueueSize(), qt.Equals, int32(3))
	<-callbackChan
	<-callbackChan
	<-callbackChan
	qt.Assert(t, d.QueueSize(), qt.Equals, int32(0))
	d.Stop()
}
