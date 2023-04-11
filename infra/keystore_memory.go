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
// @date 2017. 2. 23. AM 9:44
//

package infra

import (
	"sync"
	"throosea.com/jupiter/domain"
	"throosea.com/log"
	"time"
)

const (
	TOKEN_EXPIRE_SCANNING_TICK_SECONDS = 10
	TOKEN_EXPIRE_SECONDS               = 60
)

func newInMemoryKeyStore() *InMemoryKeyStore {
	keyStore := new(InMemoryKeyStore)
	tokenData = make(map[string]TokenAcl)

	clearTick := time.NewTicker(time.Second * TOKEN_EXPIRE_SCANNING_TICK_SECONDS)
	go func() {
		for range clearTick.C {
			keyStore.clear()
		}
	}()

	return keyStore
}

var tokenData map[string]TokenAcl

type TokenAcl struct {
	role     domain.Role
	expireAt time.Time
}

type InMemoryKeyStore struct {
	mutex sync.RWMutex
}

func (t *InMemoryKeyStore) Put(token string, role domain.Role, ttlSeconds time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	tokenData[token] = TokenAcl{role, time.Now().Add(ttlSeconds)}
}

func (t *InMemoryKeyStore) Get(token string) (role domain.Role, ok bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	acl, ok := tokenData[token]
	if !ok {
		return
	}
	role = acl.role
	return
}

func (t *InMemoryKeyStore) clear() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	for k, v := range tokenData {
		if time.Now().After(v.expireAt) {
			log.Debug("token expired : %s", k)
			delete(tokenData, k)
		}
	}
}
