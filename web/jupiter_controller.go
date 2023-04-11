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
// @date 2017. 2. 22. PM 4:24
//

package web

import (
	"mime/multipart"
	"throosea.com/jupiter/domain"
	"time"
)

type JupiterServiceController interface {
	GenerateInstantToken(role domain.Role) string
	GenerateToken(role domain.Role) string
	ValidateUser(user domain.User) (domain.Role, error)
	ValidateToken(token string, role domain.Role) error
	GetJunoEndpoint(point domain.PackagePoint, remoteAddr string) *domain.JunoPackage
	RegistJunoPackage(juno domain.JunoRegistration)
	UnregistJunoPackage(endpoint string)
	RemoveJunoPackage(endpoint string)
	GetPackageSummary(group string, location *time.Location) map[string]domain.PackageSummary
	GetEndpointList(groupName string, point domain.PackagePoint, address string) []string
	DeployPackage(mr *multipart.Reader, clientAddress string, token string) (string, error)
}
