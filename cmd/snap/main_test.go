// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/testutil"

	snap "github.com/snapcore/snapd/cmd/snap"
)

// Hook up check.v1 into the "go test" runner
func Test(t *testing.T) { TestingT(t) }

type BaseSnapSuite struct {
	testutil.BaseTest
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (s *BaseSnapSuite) SetUpTest(c *C) {
	s.BaseTest.SetUpTest(c)
	s.stdin = bytes.NewBuffer(nil)
	s.stdout = bytes.NewBuffer(nil)
	s.stderr = bytes.NewBuffer(nil)
	snap.Stdin = s.stdin
	snap.Stdout = s.stdout
	snap.Stderr = s.stderr
}

func (s *BaseSnapSuite) TearDownTest(c *C) {
	snap.Stdin = os.Stdin
	snap.Stdout = os.Stdout
	snap.Stderr = os.Stderr
	s.BaseTest.TearDownTest(c)
}

func (s *BaseSnapSuite) Stdout() string {
	return s.stdout.String()
}

func (s *BaseSnapSuite) Stderr() string {
	return s.stderr.String()
}

func (s *BaseSnapSuite) RedirectClientToTestServer(handler func(http.ResponseWriter, *http.Request)) {
	server := httptest.NewServer(http.HandlerFunc(handler))
	s.BaseTest.AddCleanup(func() { server.Close() })
	snap.ClientConfig.BaseURL = server.URL
	s.BaseTest.AddCleanup(func() { snap.ClientConfig.BaseURL = "" })
}

type SnapSuite struct {
	BaseSnapSuite
}

var _ = Suite(&SnapSuite{})

// DecodedRequestBody returns the JSON-decoded body of the request.
func DecodedRequestBody(c *C, r *http.Request) map[string]interface{} {
	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&body)
	c.Assert(err, IsNil)
	return body
}

// EncodeResponseBody writes JSON-serialized body to the response writer.
func EncodeResponseBody(c *C, w http.ResponseWriter, body interface{}) {
	encoder := json.NewEncoder(w)
	err := encoder.Encode(body)
	c.Assert(err, IsNil)
}

func (s *SnapSuite) TestErrorResult(c *C) {
	s.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"type": "error", "result": {"message": "cannot do something"}}`)
	})

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"snap", "install", "foo"}

	err := snap.RunMain()
	c.Assert(err, ErrorMatches, `cannot do something`)
}

func (s *SnapSuite) TestAccessDeniedHint(c *C) {
	s.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"type": "error", "result": {"message": "access denied", "kind": "login-required"}, "status-code": 401}`)
	})

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"snap", "install", "foo"}

	err := snap.RunMain()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Equals, `access denied (try with sudo)`)
}

func (s *SnapSuite) TestExtraArgs(c *C) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"snap", "abort", "1", "xxx", "zzz"}

	err := snap.RunMain()
	c.Assert(err, ErrorMatches, `too many arguments for command`)
}

func (s *SnapSuite) TestVersionOnClassic(c *C) {
	s.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"type":"sync","status-code":200,"status":"OK","result":{"on-classic":true,"os-release":{"id":"ubuntu","version-id":"12.34"},"series":"56","version":"7.89"}}`)
	})
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"snap", "--version"}
	err := snap.RunMain()
	c.Assert(err, IsNil)
	c.Assert(s.Stdout(), Equals,
		`snap    7.89
snapd   7.89
series  56
ubuntu  12.34
`)
	c.Assert(s.Stderr(), Equals, "")
}

func (s *SnapSuite) TestVersionOnAllSnap(c *C) {
	s.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"type":"sync","status-code":200,"status":"OK","result":{"os-release":{"id":"ubuntu","version-id":"12.34"},"series":"56","version":"7.89"}}`)
	})
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"snap", "--version"}
	err := snap.RunMain()
	c.Assert(err, IsNil)
	c.Assert(s.Stdout(), Equals,
		`snap    7.89
snapd   7.89
series  56
`)
	c.Assert(s.Stderr(), Equals, "")
}
