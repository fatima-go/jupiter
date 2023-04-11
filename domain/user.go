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
// @date 2017. 2. 22. AM 11:34
//

package domain

import (
	"strings"
	"time"
)

const (
	ROLE_MONITOR = iota
	ROLE_OPERATOR
	ROLE_UNKNOWN
)

type Role int

func (r Role) String() string {
	switch r {
	case ROLE_MONITOR:
		return "MONITOR"
	case ROLE_OPERATOR:
		return "OPERATOR"
	}
	return "UNKNOWN"
}

func (r Role) Acceptable(another Role) bool {
	if r == ROLE_UNKNOWN || another == ROLE_UNKNOWN {
		return false
	}

	switch r {
	case ROLE_OPERATOR:
		return true
	case ROLE_MONITOR:
		return another == r
	}
	return false
}

func ToRole(value string) Role {
	switch strings.ToUpper(value) {
	case "OPERATOR":
		return ROLE_OPERATOR
	case "MONITOR":
		return ROLE_MONITOR
	}
	return ROLE_UNKNOWN
}

func ToRoleString(role Role) string {
	switch role {
	case ROLE_MONITOR:
		return "MONITOR"
	case ROLE_OPERATOR:
		return "OPERATOR"
	}
	return "UNKNOWN"
}

type User struct {
	Id       string `json:"id"`
	Password string `json:"passwd"`
	Role     Role
}

type UserRepository interface {
	Save(user User)
	FindById(id string) *User
	Delete(id string)
	Exists(id string) bool
	Count() int
}

type TokenRepository interface {
	Save(token string, role Role, ttlSeconds time.Duration)
	FindById(token string) (Role, bool)
}

type Authenticate interface {
	UserAuthenticate(id, password string) (Role, error)
}

type TokenService interface {
	GenerateInstantToken(role Role) (string, error)
	GenerateToken(role Role) (string, error)
	ValidateToken(token string, role Role) error
}
