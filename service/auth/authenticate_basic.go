/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with p work for additional information
 * regarding copyright ownership.  The ASF licenses p file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use p file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 * @project fatima
 * @author DeockJin Chung (jin.freestyle@gmail.com)
 * @date 22. 10. 11. 오후 10:06
 */

package auth

import (
	"errors"
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-log"
	. "github.com/fatima-go/jupiter/domain"
	"github.com/fatima-go/jupiter/infra"
	"strings"
)

func NewBasicAuthenticator(fatimaRuntime fatima.FatimaRuntime) (Authenticate, error) {
	log.Info("creating BasicAuthenticator")
	auth := &BasicAuthenticator{}

	auth.encdec = infra.NewDefaultEncdec()
	userRepository := infra.NewMemoryUserRepository()

	repo, ok := fatimaRuntime.GetConfig().GetValue("repo")
	if ok {
		switch strings.ToLower(repo) {
		case "file":
			userRepository = infra.NewFileUserRepository(fatimaRuntime)
		}
	}

	auth.userRepository = userRepository
	return auth, nil
}

type BasicAuthenticator struct {
	userRepository UserRepository
	encdec         Encdec
	Authenticate
}

func (b *BasicAuthenticator) UserAuthenticate(id, password string) (Role, error) {
	found := b.userRepository.FindById(id)
	if found == nil {
		return ROLE_UNKNOWN, errors.New("not found user")
	}

	if b.encdec.Hash(found.Password) != b.encdec.Hash(password) {
		//if found.Password != interactor.Encdec.Hash(user.Password) {
		return ROLE_UNKNOWN, errors.New(fmt.Sprintf("missmatch password for user %s", id))
	}

	return found.Role, nil
}
