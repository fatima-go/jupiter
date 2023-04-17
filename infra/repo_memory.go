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
// @date 2017. 2. 22. PM 2:19
//

package infra

import (
	"github.com/fatima-go/jupiter/domain"
	"time"
)

func NewMemoryUserRepository() domain.UserRepository {
	return new(InMemoryUserRepository)
}

func NewMemoryTokenRepository() domain.TokenRepository {
	repo := new(InMemoryTokenRepository)
	repo.keyStore = newInMemoryKeyStore()
	return repo
}

func NewMemoryDeployRepository() domain.JunoRepository {
	return new(InMemoryJunoRepository)
}

type InMemoryUserRepository struct {
}

func (handler *InMemoryUserRepository) Save(user domain.User) {
}

func (handler *InMemoryUserRepository) FindById(id string) *domain.User {
	user := domain.User{}
	user.Id = "admin"
	user.Password = "admin"
	user.Role = domain.ROLE_OPERATOR
	return &user
}

func (handler *InMemoryUserRepository) Delete(id string) {
}

func (handler *InMemoryUserRepository) Exists(id string) bool {
	return true
}

func (handler *InMemoryUserRepository) Count() int {
	return 1
}

type InMemoryTokenRepository struct {
	keyStore KeyStore
}

func (handler *InMemoryTokenRepository) Save(token string, role domain.Role, ttlSeconds time.Duration) {
	handler.keyStore.Put(token, role, ttlSeconds)
}

func (handler *InMemoryTokenRepository) FindById(token string) (domain.Role, bool) {
	return handler.keyStore.Get(token)
}

type InMemoryJunoRepository struct {
}

func (handler *InMemoryJunoRepository) FindAll() *domain.JunoSummary {
	return new(domain.JunoSummary)
}

func (handler *InMemoryJunoRepository) FindByPoint(point domain.PackagePoint) *domain.JunoPackage {
	return nil
}

func (handler *InMemoryJunoRepository) FindByAddress(address string) *domain.JunoPackage {
	return nil
}

func (handler *InMemoryJunoRepository) FindGroup(groupName string) *domain.JunoGroup {
	return nil
}

func (handler *InMemoryJunoRepository) FindByEndpoint(endpoint string) *domain.JunoPackage {
	return nil
}

func (handler *InMemoryJunoRepository) Save(data domain.JunoRegistration) {
	return
}

func (handler *InMemoryJunoRepository) Delete(endpoint string) {
}

func (handler *InMemoryJunoRepository) SaveAll() {
	return
}
