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
// @date 2017. 2. 22. PM 2:33
//

package v1

import (
	"encoding/json"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/domain"
	"github.com/fatima-go/jupiter/web"
	"io/ioutil"
	"net/http"
)

const (
	HEADER_FATIMA_AUTH_TOKEN = "fatima-auth-token"
)

type JupiterResponse struct {
	System domain.SystemMessage `json:"system"`
}

type HandlerFunc func(web.JupiterServiceController, http.ResponseWriter, *http.Request)

func NewWebService(domainInteractor web.JupiterServiceController) web.WebServiceHandler {
	service := new(Version1Handler)
	service.controller = domainInteractor
	return service
}

type Version1Handler struct {
	controller web.JupiterServiceController
}

func (version1 *Version1Handler) GetVersion() string {
	return "v1"
}

func (version1 *Version1Handler) HandleLogin(res http.ResponseWriter, req *http.Request) {
	authorize(version1.controller, res, req)
}

func (version1 *Version1Handler) HandleToken(res http.ResponseWriter, req *http.Request) {
	version1.secureHandle(domain.ROLE_MONITOR, res, req, token)
}

func (version1 *Version1Handler) HandlePack(res http.ResponseWriter, req *http.Request) {
	version1.secureHandle(domain.ROLE_MONITOR, res, req, pack)
}

func (version1 *Version1Handler) HandleJuno(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "retrieve":
		version1.secureHandle(domain.ROLE_MONITOR, res, req, retrieveJuno)
	case "regist":
		registJuno(version1.controller, res, req)
	case "unregist":
		unregistJuno(version1.controller, res, req)
	case "remove":
		removeJuno(version1.controller, res, req)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleProc(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "regist":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, registProc)
	case "unregist":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, unregistProc)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleDeploy(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "insert":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, deployPackage)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) secureHandle(userRole domain.Role, res http.ResponseWriter, req *http.Request, businessHandler HandlerFunc) {
	token := req.Header.Get(HEADER_FATIMA_AUTH_TOKEN)
	if len(token) < 1 {
		log.Warn("Unauthorized : not found fatima token")
		web.ResponseError(res, req, http.StatusUnauthorized, "invalid access")
		return
	}

	err := version1.controller.ValidateToken(token, userRole)
	if err != nil {
		log.Warn("authorization fail :: %s :: %s", err.Error(), token)
		web.ResponseError(res, req, http.StatusUnauthorized, "invalid access")
		return
	}

	businessHandler(version1.controller, res, req)
}

func parsingRequest(req *http.Request, name string) (string, error) {
	if len(name) < 1 {
		return "", nil
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	if len(b) == 0 {
		return "", nil
	}

	params := make(map[string]string)
	err = json.Unmarshal(b, &params)
	if err != nil {
		return "", err
	}

	return params[name], nil
}

func sendSuccessResponse(res http.ResponseWriter, req *http.Request) {
	systemRes := domain.NewSuccessSystemMessage()
	jr := JupiterResponse{}
	jr.System = systemRes
	b, err := json.Marshal(jr)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func responseSuccessWithMessage(res http.ResponseWriter, req *http.Request, message string) {
	systemRes := domain.NewSuccessSystemMessage()
	systemRes.Message = message
	jr := JupiterResponse{}
	jr.System = systemRes
	b, err := json.Marshal(jr)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}
