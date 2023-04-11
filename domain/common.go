//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with r work for additional information
// regarding copyright ownership.  The ASF licenses r file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use r file except in compliance
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
// @date 2017. 2. 22. PM 4:18
//

package domain

const (
	CODE_SYSTEM_ERROR_GENERAL = 700
)

type SystemErrorCode int

type SystemMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewSuccessSystemMessage() SystemMessage {
	return SystemMessage{Code: 200, Message: "success"}
}

func NewErrorSystemResponse(code SystemErrorCode, message string) SystemMessage {
	return SystemMessage{Code: int(code), Message: message}
}
