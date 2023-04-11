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
 * @date 22. 10. 12. 오전 10:54
 */

package auth

import (
	"errors"
	"throosea.com/fatima"
	"throosea.com/fatima/lib"
	. "throosea.com/jupiter/domain"
	"throosea.com/jupiter/infra"
	"throosea.com/log"
	"time"
)

func NewTokenHelper(fatimaRuntime fatima.FatimaRuntime) TokenService {
	t := &TokenHelper{}
	t.tokenRepository = infra.NewMemoryTokenRepository()

	var err error
	var d1, d2 int
	d1, err = fatimaRuntime.GetConfig().GetInt(propTokenDurationSeconds)
	if err != nil {
		d1 = defaultTokenDurationSeconds
	}
	t.durationSeconds = time.Second * time.Duration(d1)

	d2, err = fatimaRuntime.GetConfig().GetInt(propTokenDurationInstantSeconds)
	if err != nil {
		d2 = defaultTokenDurationInstantSeconds
	}
	t.instantDurationSeconds = time.Second * time.Duration(d2)

	log.Info("duration : %d seconds, instant.duration : %d seconds", d1, d2)
	return t
}

const (
	propTokenDurationSeconds           = "token.duration.seconds"
	propTokenDurationInstantSeconds    = "token.duration.instant.seconds"
	defaultTokenDurationSeconds        = 3600
	defaultTokenDurationInstantSeconds = 10
)

type TokenHelper struct {
	tokenRepository        TokenRepository
	instantDurationSeconds time.Duration
	durationSeconds        time.Duration
}

func (t *TokenHelper) GenerateInstantToken(role Role) (string, error) {
	token := lib.RandomAlphanumeric(64)
	t.tokenRepository.Save(token, role, t.instantDurationSeconds)
	return token, nil
}

func (t *TokenHelper) GenerateToken(role Role) (string, error) {
	token := lib.RandomAlphanumeric(64)
	t.tokenRepository.Save(token, role, t.durationSeconds)
	return token, nil
}

func (t *TokenHelper) ValidateToken(token string, role Role) error {
	if len(token) < 1 {
		return errors.New("invalid fatima token")
	}

	savedRole, ok := t.tokenRepository.FindById(token)
	if !ok {
		return errors.New("not found fatima token")
	}

	if !savedRole.Acceptable(role) {
		return errors.New("insufficient previledge")
	}

	return nil
}
