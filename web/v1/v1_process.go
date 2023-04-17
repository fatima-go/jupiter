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
// @date 2017. 3. 4. PM 5:16
//

package v1

import (
	"encoding/json"
	"fmt"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/domain"
	"github.com/fatima-go/jupiter/web"
	"io/ioutil"
	"net/http"
)

const (
	JUNO_REST_PROCESS_REGIST   = "/process/regist/v1"
	JUNO_REST_PROCESS_UNREGIST = "/process/unregist/v1"
)

func registProc(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "testapp", "group_id": "4", "group": "basic", "package": "xfp-stg"}
	*/
	params, err := parsingProcRequestParam(req)
	if err != nil || len(params.GroupId) == 0 {
		var info string
		if err != nil {
			info = err.Error()
		} else {
			info = "not found group id"
		}
		log.Warn("invalid request data : %s", info)
		web.ResponseError(res, req, http.StatusBadRequest, info)
		return
	}

	log.Debug("proc regist request : %s", params)
	point := domain.NewPackagePoint(params.Package)
	endpointList := controller.GetEndpointList(params.Group, point, params.ClientAddress)
	log.Debug("list : %s", endpointList)

	httpClient := web.NewHttpClient(req)
	procReq := domain.ProcRequest{Process: params.Process, GroupId: params.GroupId}
	data, err := json.Marshal(procReq)
	if err != nil {
		log.Warn("fail to prepare http client : %s", err.Error())
		return
	}

	total := len(endpointList)
	success := 0
	var resp []byte
	for _, endpoint := range endpointList {
		resp, err = httpClient.Post(buildRestUrl(endpoint, JUNO_REST_PROCESS_REGIST), data)
		if err != nil {
			log.Warn("fail to call endpoint[%s] : %s", endpoint, err.Error())
			continue
		}
		success = success + 1
		log.Debug("resp : %s", string(resp))
	}

	/*
		{"system": {"message": "total 1 juno. process 1 registed", "code": 200}}
	*/
	message := fmt.Sprintf("Regist juno. total=%d, success=%d", total, success)
	responseSuccessWithMessage(res, req, message)
}

func buildRestUrl(endpoint string, suffix string) string {
	var url string
	if endpoint[len(endpoint)-1] == '/' {
		if suffix[0] == '/' {
			url = endpoint + suffix[1:]
		} else {
			url = endpoint + suffix
		}
	} else {
		if suffix[0] == '/' {
			url = endpoint + suffix
		} else {
			url = endpoint + "/" + suffix
		}
	}
	return url
}

func unregistProc(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "testapp", "group": "basic", "package": "xfp-stg"}
	*/
	params, err := parsingProcRequestParam(req)
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	log.Debug("proc unregist param : %s", params)
	point := domain.NewPackagePoint(params.Package)
	endpointList := controller.GetEndpointList(params.Group, point, params.ClientAddress)
	log.Debug("list : %s", endpointList)

	httpClient := web.NewHttpClient(req)
	procReq := domain.ProcRequest{Process: params.Process, GroupId: params.GroupId}
	data, err := json.Marshal(procReq)
	if err != nil {
		log.Warn("fail to prepare http client : %s", err.Error())
		return
	}

	total := len(endpointList)
	success := 0
	var resp []byte
	for _, endpoint := range endpointList {
		resp, err = httpClient.PostWithTimeout(buildRestUrl(endpoint, JUNO_REST_PROCESS_UNREGIST), data, 13)
		if err != nil {
			log.Warn("fail to call endpoint[%s] : %s", endpoint, err.Error())
			continue
		}
		success = success + 1
		log.Debug("resp : %s", string(resp))
	}

	/*
		{"system": {"message": "total 1 juno. 1 unregisted", "code": 200}}
	*/
	message := fmt.Sprintf("UnRegist juno. total=%d, success=%d", total, success)
	responseSuccessWithMessage(res, req, message)
}

func parsingProcRequestParam(req *http.Request) (*domain.ProcRequest, error) {
	var data domain.ProcRequest

	b, _ := ioutil.ReadAll(req.Body)
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("fail to parse data : %s", err)
	}

	data.ClientAddress = req.RemoteAddr
	return &data, nil
}
