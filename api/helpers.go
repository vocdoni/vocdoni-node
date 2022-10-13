package api

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/proto/build/go/models"
)

func (a *API) getProcessSummaryList(pids ...[]byte) ([]*ElectionSummary, error) {
	processes := []*ElectionSummary{}
	for _, p := range pids {
		procInfo, err := a.scrutinizer.ProcessInfo(p)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch election info: %w", err)
		}
		processes = append(processes, &ElectionSummary{
			ElectionID: procInfo.ID,
			Status:     models.ProcessStatus_name[procInfo.Status],
			StartDate:  procInfo.CreationTime,
			EndDate:    a.vocinfo.HeightTime(int64(procInfo.EndBlock)),
		})
	}
	return processes, nil
}

func (a *API) formatElectionType(et *models.EnvelopeType) string {
	ptype := strings.Builder{}

	if et.Anonymous {
		ptype.WriteString("anonymous")
	} else {
		ptype.WriteString("poll")
	}
	if et.EncryptedVotes {
		ptype.WriteString(" encrypted")
	} else {
		ptype.WriteString(" open")
	}
	if et.Serial {
		ptype.WriteString(" serial")
	} else {
		ptype.WriteString(" single")
	}
	return ptype.String()
}

type testHTTPclient struct {
	c     *http.Client
	token *uuid.UUID
	addr  *url.URL
	t     *testing.T
}

func (c *testHTTPclient) request(method string, body []byte, urlPath ...string) ([]byte, int) {
	u, err := url.Parse(c.addr.String())
	qt.Assert(c.t, err, qt.IsNil)
	u.Path = path.Join(u.Path, path.Join(urlPath...))
	headers := http.Header{}
	if c.token != nil {
		headers = http.Header{"Authorization": []string{"Bearer " + c.token.String()}}
	}
	resp, err := c.c.Do(&http.Request{
		Method: method,
		URL:    u,
		Header: headers,
		Body:   io.NopCloser(bytes.NewBuffer(body)),
	})
	qt.Assert(c.t, err, qt.IsNil)
	data, err := ioutil.ReadAll(resp.Body)
	qt.Assert(c.t, err, qt.IsNil)
	return data, resp.StatusCode
}

func newTestHTTPclient(t *testing.T, addr *url.URL, bearerToken *uuid.UUID) *testHTTPclient {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    5 * time.Second,
		DisableCompression: false,
	}
	return &testHTTPclient{
		c:     &http.Client{Transport: tr, Timeout: time.Second * 8},
		token: bearerToken,
		addr:  addr,
		t:     t,
	}
}
