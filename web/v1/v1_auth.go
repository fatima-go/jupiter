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
// @date 2017. 2. 22. PM 3:53
//

package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"throosea.com/jupiter/domain"
	"throosea.com/jupiter/web"
	"throosea.com/log"
)

func authorize(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	user, err := parsingUser(req)
	if err != nil {
		log.Warn("validation fail : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	var role domain.Role
	role, err = controller.ValidateUser(*user)
	if err != nil {
		log.Warn("unauthorized : %s, %s", user.Id, err.Error())
		web.ResponseError(res, req, http.StatusUnauthorized, "user authorization fail")
		return
	}

	var userToken string
	if isFatimaClientCli(req) {
		userToken = controller.GenerateInstantToken(role)
	} else {
		userToken = controller.GenerateToken(role)
	}
	log.Debug("user[%s] token generated : %s", user.Id, userToken)

	vars := make(map[string]string)
	vars["token"] = userToken
	b, err := json.Marshal(vars)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, "system error")
		return
	}

	web.ResponseSuccess(res, req, string(b))
}

func parsingUser(req *http.Request) (*domain.User, error) {
	var user domain.User

	b, _ := io.ReadAll(req.Body)
	if err := json.Unmarshal(b, &user); err != nil {
		return nil, fmt.Errorf("fail to parse data : %s", err)
	}

	if len(user.Id) < 3 || len(user.Password) < 3 {
		return nil, errors.New("unknown id or invalid password")
	}
	return &user, nil
}

func isFatimaClientCli(req *http.Request) bool {
	userAgent := req.Header.Get("user-agent")
	return userAgent == UserAgentFatimaCli
}

const (
	UserAgentFatimaCli = "go-fatimaclient"
)
