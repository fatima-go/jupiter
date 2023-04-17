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
// @date 2017. 3. 15. PM 1:55
//

package v1

import (
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/web"
	"mime"
	"mime/multipart"
	"net/http"
)

func deployPackage(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	// https://golang.org/pkg/mime/multipart/#example_NewReader

	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}

	token := web.GetFatimaAuthToken(req)

	var result string
	mr := multipart.NewReader(req.Body, params["boundary"])
	result, err = controller.DeployPackage(mr, req.RemoteAddr, token)
	if err != nil {
		log.Warn("fail to deploy : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}

	web.ResponseSystemSuccess(res, req, result)
}
