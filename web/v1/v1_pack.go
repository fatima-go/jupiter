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
// @date 2017. 3. 4. PM 4:11
//

package v1

import (
	"encoding/json"
	"net/http"
	"throosea.com/jupiter/web"
	"throosea.com/log"
)

func pack(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	group, err := parsingRequest(req, "group")
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	location := web.GetFatimaClientTimezone(req)
	report := controller.GetPackageSummary(group, location)
	log.Debug("report : %s", report)
	b, err := json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}
