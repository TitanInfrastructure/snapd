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

package client_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strconv"

	"gopkg.in/check.v1"

	"github.com/snapcore/snapd/client"
)

var chanName = "achan"

var ops = []struct {
	op     func(*client.Client, string, *client.SnapOptions) (string, error)
	action string
}{
	{(*client.Client).Install, "install"},
	{(*client.Client).Refresh, "refresh"},
	{(*client.Client).Remove, "remove"},
	{(*client.Client).Revert, "revert"},
	{(*client.Client).Enable, "enable"},
	{(*client.Client).Disable, "disable"},
}

var multiOps = []struct {
	op     func(*client.Client, []string, *client.SnapOptions) (string, error)
	action string
}{
	{(*client.Client).RefreshMany, "refresh"},
}

func (cs *clientSuite) TestClientOpSnapServerError(c *check.C) {
	cs.err = errors.New("fail")
	for _, s := range ops {
		_, err := s.op(cs.cli, pkgName, nil)
		c.Check(err, check.ErrorMatches, `.*fail`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientMultiOpSnapServerError(c *check.C) {
	cs.err = errors.New("fail")
	for _, s := range multiOps {
		_, err := s.op(cs.cli, nil, nil)
		c.Check(err, check.ErrorMatches, `.*fail`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpSnapResponseError(c *check.C) {
	cs.rsp = `{"type": "error", "status": "potatoes"}`
	for _, s := range ops {
		_, err := s.op(cs.cli, pkgName, nil)
		c.Check(err, check.ErrorMatches, `.*server error: "potatoes"`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientMultiOpSnapResponseError(c *check.C) {
	cs.rsp = `{"type": "error", "status": "potatoes"}`
	for _, s := range multiOps {
		_, err := s.op(cs.cli, nil, nil)
		c.Check(err, check.ErrorMatches, `.*server error: "potatoes"`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpSnapBadType(c *check.C) {
	cs.rsp = `{"type": "what"}`
	for _, s := range ops {
		_, err := s.op(cs.cli, pkgName, nil)
		c.Check(err, check.ErrorMatches, `.*expected async response for "POST" on "/v2/snaps/`+pkgName+`", got "what"`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpSnapNotAccepted(c *check.C) {
	cs.rsp = `{
		"status-code": 200,
		"type": "async"
	}`
	for _, s := range ops {
		_, err := s.op(cs.cli, pkgName, nil)
		c.Check(err, check.ErrorMatches, `.*operation not accepted`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpSnapNoChange(c *check.C) {
	cs.rsp = `{
		"status-code": 202,
		"type": "async"
	}`
	for _, s := range ops {
		_, err := s.op(cs.cli, pkgName, nil)
		c.Assert(err, check.ErrorMatches, `.*response without change reference.*`, check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpSnap(c *check.C) {
	cs.rsp = `{
		"change": "d728",
		"status-code": 202,
		"type": "async"
	}`
	for _, s := range ops {
		id, err := s.op(cs.cli, pkgName, &client.SnapOptions{Channel: chanName})
		c.Assert(err, check.IsNil)

		c.Assert(cs.req.Header.Get("Content-Type"), check.Equals, "application/json", check.Commentf(s.action))

		body, err := ioutil.ReadAll(cs.req.Body)
		c.Assert(err, check.IsNil, check.Commentf(s.action))
		jsonBody := make(map[string]string)
		err = json.Unmarshal(body, &jsonBody)
		c.Assert(err, check.IsNil, check.Commentf(s.action))
		c.Check(jsonBody["action"], check.Equals, s.action, check.Commentf(s.action))
		c.Check(jsonBody["channel"], check.Equals, chanName, check.Commentf(s.action))
		c.Check(jsonBody, check.HasLen, 2, check.Commentf(s.action))

		c.Check(cs.req.URL.Path, check.Equals, fmt.Sprintf("/v2/snaps/%s", pkgName), check.Commentf(s.action))
		c.Check(id, check.Equals, "d728", check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientMultiOpSnap(c *check.C) {
	cs.rsp = `{
		"change": "d728",
		"status-code": 202,
		"type": "async"
	}`
	for _, s := range multiOps {
		id, err := s.op(cs.cli, []string{pkgName}, nil)
		c.Assert(err, check.IsNil)

		c.Assert(cs.req.Header.Get("Content-Type"), check.Equals, "application/json", check.Commentf(s.action))

		body, err := ioutil.ReadAll(cs.req.Body)
		c.Assert(err, check.IsNil, check.Commentf(s.action))
		jsonBody := make(map[string]interface{})
		err = json.Unmarshal(body, &jsonBody)
		c.Assert(err, check.IsNil, check.Commentf(s.action))
		c.Check(jsonBody["action"], check.Equals, s.action, check.Commentf(s.action))
		c.Check(jsonBody["snaps"], check.DeepEquals, []interface{}{pkgName}, check.Commentf(s.action))
		c.Check(jsonBody, check.HasLen, 2, check.Commentf(s.action))

		c.Check(cs.req.URL.Path, check.Equals, "/v2/snaps", check.Commentf(s.action))
		c.Check(id, check.Equals, "d728", check.Commentf(s.action))
	}
}

func (cs *clientSuite) TestClientOpInstallPath(c *check.C) {
	cs.rsp = `{
		"change": "66b3",
		"status-code": 202,
		"type": "async"
	}`
	bodyData := []byte("snap-data")

	snap := filepath.Join(c.MkDir(), "foo.snap")
	err := ioutil.WriteFile(snap, bodyData, 0644)
	c.Assert(err, check.IsNil)

	id, err := cs.cli.InstallPath(snap, nil)
	c.Assert(err, check.IsNil)

	body, err := ioutil.ReadAll(cs.req.Body)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), check.Matches, "(?s).*\r\nsnap-data\r\n.*")
	c.Assert(string(body), check.Matches, "(?s).*Content-Disposition: form-data; name=\"action\"\r\n\r\ninstall\r\n.*")
	c.Assert(string(body), check.Matches, "(?s).*Content-Disposition: form-data; name=\"force-dangerous\"\r\n\r\nfalse\r\n.*")

	c.Check(cs.req.Method, check.Equals, "POST")
	c.Check(cs.req.URL.Path, check.Equals, fmt.Sprintf("/v2/snaps"))
	c.Assert(cs.req.Header.Get("Content-Type"), check.Matches, "multipart/form-data; boundary=.*")
	c.Check(id, check.Equals, "66b3")
}

func formToMap(c *check.C, mr *multipart.Reader) map[string]string {
	formData := map[string]string{}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		c.Assert(err, check.IsNil)
		slurp, err := ioutil.ReadAll(p)
		c.Assert(err, check.IsNil)
		formData[p.FormName()] = string(slurp)
	}
	return formData
}

func (cs *clientSuite) TestClientOpTryMode(c *check.C) {
	cs.rsp = `{
		"change": "66b3",
		"status-code": 202,
		"type": "async"
	}`
	snapdir := filepath.Join(c.MkDir(), "/some/path")

	for _, opts := range []*client.SnapOptions{
		{DevMode: false, JailMode: false},
		{DevMode: false, JailMode: true},
		{DevMode: true, JailMode: true},
		{DevMode: true, JailMode: false},
	} {
		id, err := cs.cli.Try(snapdir, opts)
		c.Assert(err, check.IsNil)

		// ensure we send the right form-data
		_, params, err := mime.ParseMediaType(cs.req.Header.Get("Content-Type"))
		c.Assert(err, check.IsNil)
		mr := multipart.NewReader(cs.req.Body, params["boundary"])
		formData := formToMap(c, mr)
		c.Check(formData, check.DeepEquals, map[string]string{
			"action":    "try",
			"snap-path": snapdir,
			"devmode":   strconv.FormatBool(opts.DevMode),
			"jailmode":  strconv.FormatBool(opts.JailMode),
		})

		c.Check(cs.req.Method, check.Equals, "POST")
		c.Check(cs.req.URL.Path, check.Equals, fmt.Sprintf("/v2/snaps"))
		c.Assert(cs.req.Header.Get("Content-Type"), check.Matches, "multipart/form-data; boundary=.*")
		c.Check(id, check.Equals, "66b3")
	}
}
