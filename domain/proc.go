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
// @date 2017. 3. 5. AM 12:03
//

package domain

type ProcRequest struct {
	Process       string `json:"process"`
	GroupId       string `json:"group_id,omitempty"`
	Group         string `json:"group,omitempty"`
	Package       string `json:"package,omitempty"`
	ClientAddress string
}
