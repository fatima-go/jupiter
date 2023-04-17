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
// @date 2017. 2. 22. PM 2:02
//

package web

import (
	"encoding/json"
	"fmt"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/domain"
	"net/http"
	"time"
)

const (
	HeaderAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods = "Access-Control-Allow-Methods"
	HeaderAccessControlMaxAge       = "Access-Control-Max-Age"
	HeaderAccessControlAllowHeaders = "Access-Control-Allow-Headers"
	HeaderFatimaAuthToken           = "Fatima-Auth-Token"
	HeaderContentType               = "Content-Type"
	HeaderCharset                   = "Charset"
	HeaderUserAgent                 = "User-Agent"
	HeaderFatimaTimezone            = "Fatima-Timezone"
	HeaderFatimaResTime             = "Fatima-Response-Time"
	HeaderFatimaTokenRole           = "Fatima-Token-Role"

	HeaderValueUserAgent   = "fatima-application-jupiter"
	HeaderValueCharset     = "UTF-8"
	HeaderValueContentType = "application/json; charset=utf-8"

	TIME_YYYYMMDDHHMMSS = "2006-01-02 15:04:05"
)

type ServerError struct {
	Message string `json:"message"`
}

func GetFatimaAuthToken(req *http.Request) string {
	if token, ok := req.Header[HeaderFatimaAuthToken]; ok {
		return token[0]
	}

	return ""
}

func writeResponseHeader(res http.ResponseWriter, req *http.Request, httpStatusCode int) {
	res.Header().Set(HeaderAccessControlAllowOrigin, "*")
	res.Header().Set(HeaderContentType, HeaderValueContentType)
	res.Header().Set(HeaderCharset, HeaderValueCharset)
	res.Header().Set(HeaderUserAgent, HeaderValueUserAgent)
	if tz, ok := req.Header[HeaderFatimaTimezone]; ok {
		if loc, err := time.LoadLocation(tz[0]); err == nil {
			res.Header().Set(HeaderFatimaTimezone, tz[0])
			res.Header().Set(HeaderFatimaResTime, time.Now().In(loc).Format(TIME_YYYYMMDDHHMMSS))
		}
	}
	res.WriteHeader(httpStatusCode)
}

func GetFatimaClientTimezone(req *http.Request) *time.Location {
	if tz, ok := req.Header[HeaderFatimaTimezone]; ok {
		if loc, err := time.LoadLocation(tz[0]); err == nil {
			return loc
		}
	}

	// default
	return time.UTC
}

func ResponseSuccess(res http.ResponseWriter, req *http.Request, message string) {
	writeResponseHeader(res, req, http.StatusOK)
	if len(message) > 0 {
		fmt.Fprintln(res, message)
	}
}

func ResponseError(res http.ResponseWriter, req *http.Request, httpStatusCode int, message string) {
	writeResponseHeader(res, req, httpStatusCode)

	if len(message) > 0 {
		errorResponse := ServerError{message}
		outgoingJSON, err := json.Marshal(errorResponse)
		if err != nil {
			log.Warn("fail to make response", err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(res, string(outgoingJSON))
	}
}

func ResponseSystemSuccess(res http.ResponseWriter, req *http.Request, message string) {
	// {'system': {'message': 'not found process : benefita1p', 'code': 700}}
	system := make(map[string]domain.SystemMessage)
	report := domain.NewSuccessSystemMessage()
	report.Message = message
	system["system"] = report
	b, err := json.Marshal(system)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(res, req, string(b))
}
