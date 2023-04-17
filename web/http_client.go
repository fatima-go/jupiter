//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project jupiter
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 3. 4. PM 7:11
//

package web

import (
	"bytes"
	"errors"
	"github.com/fatima-go/jupiter/domain"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

var netTransport = &http.Transport{
	Dial: (&net.Dialer{
		Timeout: 2 * time.Second,
	}).Dial,
	TLSHandshakeTimeout: 2 * time.Second,
}

var netClient = &http.Client{
	Timeout:   time.Second * 60,
	Transport: netTransport,
}

type HttpClient struct {
	bare        bool
	token       string
	timezone    string
	contentType string
}

func NewHttpClient(req *http.Request) HttpClient {
	client := HttpClient{bare: true}
	if req != nil {
		client.bare = false
		client.token = GetFatimaAuthToken(req)
		client.timezone = GetFatimaClientTimezone(req).String()
	}
	return client
}

func (hc *HttpClient) SetToken(token string) {
	hc.token = token
}

func (hc *HttpClient) SetContentType(contentType string) {
	hc.contentType = contentType
}

func (hc HttpClient) Post(url string, body []byte) ([]byte, error) {
	return hc.post(netClient, url, body)
}

func (hc HttpClient) PostWithTimeout(url string, body []byte, timeoutSeconds int) ([]byte, error) {
	var netClient = &http.Client{
		Timeout:   time.Second * time.Duration(timeoutSeconds),
		Transport: netTransport,
	}

	return hc.post(netClient, url, body)
}

func (hc HttpClient) post(netClient *http.Client, url string, body []byte) ([]byte, error) {
	if !hc.bare {
		if len(hc.token) == 0 {
			return nil, errors.New("need token")
		}
		if len(hc.timezone) == 0 {
			return nil, errors.New("need timezone")
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add(HeaderUserAgent, HeaderValueUserAgent)
	req.Header.Add(HeaderCharset, HeaderValueCharset)
	if len(hc.contentType) == 0 {
		req.Header.Add(HeaderContentType, HeaderValueContentType)
	} else {
		req.Header.Add(HeaderContentType, hc.contentType)
	}

	if len(hc.token) > 0 {
		req.Header.Add(HeaderFatimaAuthToken, hc.token)
		req.Header.Add(HeaderFatimaTokenRole, domain.ToRoleString(domain.ROLE_OPERATOR))
	}
	if len(hc.timezone) > 0 {
		req.Header.Add(HeaderFatimaTimezone, hc.timezone)
	}

	var resp *http.Response

	resp, err = netClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (hc HttpClient) PostWithBuffer(url string, body *bytes.Buffer) ([]byte, error) {
	if !hc.bare {
		if len(hc.token) == 0 {
			return nil, errors.New("need token")
		}
		if len(hc.timezone) == 0 {
			return nil, errors.New("need timezone")
		}
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add(HeaderUserAgent, HeaderValueUserAgent)
	req.Header.Add(HeaderCharset, HeaderValueCharset)
	if len(hc.contentType) == 0 {
		req.Header.Add(HeaderContentType, HeaderValueContentType)
	} else {
		req.Header.Add(HeaderContentType, hc.contentType)
	}

	if len(hc.token) > 0 {
		req.Header.Add(HeaderFatimaAuthToken, hc.token)
		req.Header.Add(HeaderFatimaTokenRole, domain.ToRoleString(domain.ROLE_OPERATOR))
	}
	if len(hc.timezone) > 0 {
		req.Header.Add(HeaderFatimaTimezone, hc.timezone)
	}

	var resp *http.Response
	resp, err = netClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
