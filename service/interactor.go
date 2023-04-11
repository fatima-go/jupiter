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
// @date 2017. 2. 22. PM 1:21
//

package service

import (
	"fmt"
	"throosea.com/fatima"
	"throosea.com/jupiter/domain"
	"throosea.com/jupiter/infra"
	"throosea.com/jupiter/service/auth"
)

const (
	propAuth       = "auth"
	valueAuthBasic = "basic"
	valueAuthLdap  = "ldap"
)

func NewDomainInteractor(fatimaRuntime fatima.FatimaRuntime) (*DomainInteractor, error) {
	domainInteractor := new(DomainInteractor)
	domainInteractor.fatimaRuntime = fatimaRuntime
	domainInteractor.tokenService = auth.NewTokenHelper(fatimaRuntime)
	domainInteractor.JunoRepository = infra.NewFileJunoRepository(fatimaRuntime)

	var err error

	// auth=basic
	//auth.ldap.helper.ip=127.0.0.1
	//auth.ldap.helper.port=6413
	authMethod, err := fatimaRuntime.GetConfig().GetString(propAuth)
	if err != nil {
		authMethod = valueAuthBasic
	}

	switch authMethod {
	case valueAuthBasic:
		domainInteractor.authenticator, err = auth.NewBasicAuthenticator(fatimaRuntime)
		if err != nil {
			return domainInteractor, err
		}
	case valueAuthLdap:
		domainInteractor.authenticator, err = auth.NewLdapAuthenticator(fatimaRuntime)
		if err != nil {
			return domainInteractor, err
		}
	default:
		return domainInteractor, fmt.Errorf("unknown auth method %s", authMethod)
	}

	return domainInteractor, nil
}

type DomainInteractor struct {
	fatimaRuntime  fatima.FatimaRuntime
	authenticator  domain.Authenticate
	tokenService   domain.TokenService
	JunoRepository domain.JunoRepository
}

func (interactor *DomainInteractor) ValidateUser(user domain.User) (domain.Role, error) {
	return interactor.authenticator.UserAuthenticate(user.Id, user.Password)
}

func (interactor *DomainInteractor) GenerateToken(role domain.Role) string {
	token, _ := interactor.tokenService.GenerateToken(role)
	return token
}

func (interactor *DomainInteractor) GenerateInstantToken(role domain.Role) string {
	token, _ := interactor.tokenService.GenerateInstantToken(role)
	return token
}

func (interactor *DomainInteractor) ValidateToken(token string, role domain.Role) error {
	return interactor.tokenService.ValidateToken(token, role)
}
