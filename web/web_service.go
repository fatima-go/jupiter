//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with handler work for additional information
// regarding copyright ownership.  The ASF licenses handler file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use handler file except in compliance
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
// @date 2017. 2. 22. PM 1:32
//

package web

import (
	"github.com/gorilla/mux"
	//"throosea.com/jupiter/domain"
	"net/http"
)

type WebService struct {
	versions map[string]WebServiceHandler
}

var webService *WebService

func init() {
	webService = new(WebService)
	webService.versions = make(map[string]WebServiceHandler, 0)
}

func GetWebService() *WebService {
	return webService
}

type WebServiceHandler interface {
	GetVersion() string
	HandleLogin(res http.ResponseWriter, req *http.Request)
	HandleToken(res http.ResponseWriter, req *http.Request)
	HandlePack(res http.ResponseWriter, req *http.Request)
	HandleJuno(method string, res http.ResponseWriter, req *http.Request)
	HandleProc(method string, res http.ResponseWriter, req *http.Request)
	HandleDeploy(method string, res http.ResponseWriter, req *http.Request)
}

func (handler *WebService) Regist(service WebServiceHandler) {
	handler.versions[service.GetVersion()] = service
}

func (handler *WebService) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	writeCORSResponse(res, req)
}

func (handler *WebService) GenerateSubRouter(router *mux.Router) {
	router.PathPrefix("/").
		Methods("OPTIONS").
		Handler(handler)

	subrouter := router.PathPrefix("/auth").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/login/{version}", handler.Login)

	subrouter = router.PathPrefix("/token").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/{version}", handler.Token)

	subrouter = router.PathPrefix("/pack").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/{version}", handler.Pack)

	subrouter = router.PathPrefix("/juno").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/{method}/{version}", handler.Juno)

	subrouter = router.PathPrefix("/proc").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/{method}/{version}", handler.Proc)

	subrouter = router.PathPrefix("/deploy").
		Methods("POST").
		HeadersRegexp("Content-Type", "multipart*").
		Subrouter()

	subrouter.HandleFunc("/{method}/{version}", handler.Deploy)
}

// var AccessControlAllowHeaderList = "Content-Type, Access-Control-Allow-Headers, Authorization, Fatima-Auth-Token, Fatima-Timezone"
var AccessControlAllowHeaderList = "Content-Type, Fatima-Auth-Token, Fatima-Timezone"

func writeCORSResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(HeaderAccessControlAllowOrigin, "*")
	res.Header().Set(HeaderAccessControlAllowMethods, "POST, GET, OPTIONS")
	res.Header().Set(HeaderAccessControlMaxAge, "86400")
	res.Header().Set(HeaderAccessControlAllowHeaders, AccessControlAllowHeaderList)
	//res.Header().Add("Vary", "Origin")
	//res.Header().Add("Vary", "Access-Control-Request-Method")
	//res.Header().Add("Vary", "Access-Control-Request-Headers")
	res.WriteHeader(http.StatusOK)
}

func (handler *WebService) Login(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	service.HandleLogin(res, req)
}

func (handler *WebService) Token(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	service.HandleToken(res, req)
}

func (handler *WebService) Pack(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	service.HandlePack(res, req)
}

func (handler *WebService) Juno(res http.ResponseWriter, req *http.Request) {
	var method string
	var ok bool

	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	method, ok = vars["method"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "resouce path not found")
		return
	}

	service.HandleJuno(method, res, req)
}

func (handler *WebService) Proc(res http.ResponseWriter, req *http.Request) {
	var method string
	var ok bool

	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	method, ok = vars["method"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "resouce path not found")
		return
	}

	service.HandleProc(method, res, req)
}

func (handler *WebService) Deploy(res http.ResponseWriter, req *http.Request) {
	var method string
	var ok bool

	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	method, ok = vars["method"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "resouce path not found")
		return
	}

	service.HandleDeploy(method, res, req)
}
