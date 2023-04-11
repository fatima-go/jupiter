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
// @date 2017. 2. 23. PM 3:12
//

package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"throosea.com/jupiter/domain"
	"throosea.com/jupiter/web"
	"throosea.com/log"
	"time"
)

type JunoResponse struct {
	Endpoint string `json:"endpoint,omitempty"`
	JupiterResponse
}

type JunoRegistParam struct {
	Group    string              `json:"package_group"`
	Host     string              `json:"package_host"`
	Name     string              `json:"package_name"`
	Endpoint string              `json:"endpoint"`
	Platform domain.PlatformInfo `json:"platform"`
}

func (handler *JunoRegistParam) ToJunoRegistration() domain.JunoRegistration {
	juno := domain.JunoRegistration{}
	juno.Group = handler.Group
	juno.Host = handler.Host
	juno.Name = handler.Name
	juno.Endpoint = handler.Endpoint
	juno.Status = domain.JUNO_STATUS_ALIVE
	juno.RegistDate = time.Now().Unix()
	juno.Platform = handler.Platform
	return juno
}

func sendJunoResponse(res http.ResponseWriter, req *http.Request, junoRes JunoResponse) {
	b, err := json.Marshal(junoRes)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func sendJunoSuccessResponse(res http.ResponseWriter, req *http.Request) {
	systemRes := domain.NewSuccessSystemMessage()
	jr := JunoResponse{}
	jr.System = systemRes
	b, err := json.Marshal(jr)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func retrieveJuno(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	pack, err := parsingRequest(req, "package")
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	point := domain.NewPackagePoint(pack)
	juno := controller.GetJunoEndpoint(point, req.RemoteAddr)
	if juno == nil {
		if len(pack) > 0 {
			message := fmt.Sprintf("host(package) %s does not exist", pack)
			systemRes := domain.NewErrorSystemResponse(domain.CODE_SYSTEM_ERROR_GENERAL, message)
			jr := JunoResponse{}
			jr.System = systemRes
			sendJunoResponse(res, req, jr)
			return
		}
		message := fmt.Sprintf("there are many host(package) exist. you have to specify host:package with option -p")
		systemRes := domain.NewErrorSystemResponse(domain.CODE_SYSTEM_ERROR_GENERAL, message)
		jr := JunoResponse{}
		jr.System = systemRes
		sendJunoResponse(res, req, jr)
		return
	}

	systemRes := domain.NewSuccessSystemMessage()
	jr := JunoResponse{Endpoint: juno.Endpoint}
	jr.System = systemRes
	sendJunoResponse(res, req, jr)
}

func registJuno(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	junoParam, err := parsingJunoRegistParam(req)
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("juno regist candidate : %s", junoParam)
	controller.RegistJunoPackage(junoParam.ToJunoRegistration())
	sendJunoSuccessResponse(res, req)
}

func unregistJuno(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	endpoint, err := parsingRequest(req, "endpoint")
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("try to unregist endpoint : %s", endpoint)
	controller.UnregistJunoPackage(endpoint)
	sendJunoSuccessResponse(res, req)
}

func removeJuno(controller web.JupiterServiceController, res http.ResponseWriter, req *http.Request) {
	endpoint, err := parsingRequest(req, "endpoint")
	if err != nil {
		log.Warn("invalid request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("try to remove endpoint : %s", endpoint)
	controller.RemoveJunoPackage(endpoint)
	sendJunoSuccessResponse(res, req)
}

func parsingJunoRegistParam(req *http.Request) (*JunoRegistParam, error) {
	var data JunoRegistParam

	b, _ := io.ReadAll(req.Body)
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("fail to parse data : %s", err)
	}

	return &data, nil
}
