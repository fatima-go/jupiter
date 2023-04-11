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
 * @date 22. 10. 12. 오후 8:19
 */

package auth

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"throosea.com/fatima"
	. "throosea.com/jupiter/domain"
	proto "throosea.com/jupiter/proto/ldap.adapter.v1"
	"throosea.com/log"
	"time"
)

func NewLdapAuthenticator(fatimaRuntime fatima.FatimaRuntime) (Authenticate, error) {
	log.Info("creating LdapAuthenticator")
	auth := &LdapAuthenticator{}

	var err error
	auth.ldapHelperAddress, err = fatimaRuntime.GetConfig().GetString(propAuthLdapHelperIp)
	if err != nil {
		auth.ldapHelperAddress = defaultAuthLdapHelperIp
	}
	auth.ldapHelperPort, err = fatimaRuntime.GetConfig().GetInt(propAuthLdapHelperPort)
	if err != nil {
		auth.ldapHelperPort = defaultAuthLdapHelperPort
	}

	log.Info("Using ldap helper %s:%d", auth.ldapHelperAddress, auth.ldapHelperPort)

	auth.connectToHelper() // we don't need check error response

	return auth, nil
}

// auth=basic
// auth.ldap.helper.ip=127.0.0.1
// auth.ldap.helper.port=6413
const (
	propAuthLdapHelperIp      = "auth.ldap.helper.ip"
	propAuthLdapHelperPort    = "auth.ldap.helper.port"
	defaultAuthLdapHelperIp   = "127.0.0.1"
	defaultAuthLdapHelperPort = 6413
)

type LdapAuthenticator struct {
	ldapHelperAddress string
	ldapHelperPort    int
	conn              *grpc.ClientConn
	mu                sync.Mutex
	Authenticate
}

func (l *LdapAuthenticator) connectToHelper() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.conn != nil {
		return nil // maybe exist available connection
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	gConn, err := grpc.DialContext(
		ctx,
		fmt.Sprintf("%s:%d", l.ldapHelperAddress, l.ldapHelperPort),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("fail to connect saturn : %s", err.Error())
	}

	log.Info("connected to ldap helper")
	l.conn = gConn
	return nil
}

func (l *LdapAuthenticator) UserAuthenticate(id, password string) (Role, error) {
	log.Info("Ask id=%s", id)
	if l.conn == nil {
		err := l.connectToHelper()
		if err != nil {
			return ROLE_UNKNOWN, fmt.Errorf("userAuthenticate::connectToHelper error : %s", err.Error())
		}
	}

	req := proto.AuthenticateRequest{}
	req.Id = id
	req.Password = password

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := proto.NewLdapAdapterServiceClient(l.conn).Authenticate(ctx, &req)
	if err != nil {
		l.conn.Close()
		l.conn = nil
		return ROLE_UNKNOWN, fmt.Errorf("userAuthenticate::Authenticate error : %s", err.Error())
	}

	if errRes, ok := res.Response.(*proto.AuthenticateResponse_Error); ok {
		switch errRes.Error.GrpcResponse {
		case proto.ResponseError_UNAUTORIZED:
			log.Warn("UNAUTORIZED : [%s] %s", errRes.Error.Code, errRes.Error.Desc)
			return ROLE_UNKNOWN, errors.New(fmt.Sprintf("unauthorized for user %s", id))
		case proto.ResponseError_BAD_PARAMETER:
			log.Warn("BAD_PARAMETER : [%s] %s", errRes.Error.Code, errRes.Error.Desc)
			return ROLE_UNKNOWN, errors.New(fmt.Sprintf("bad parameter for user %s", id))
		case proto.ResponseError_NOT_FOUND:
			log.Warn("NOT_FOUND : [%s] %s", errRes.Error.Code, errRes.Error.Desc)
			return ROLE_UNKNOWN, errors.New(fmt.Sprintf("not found for user %s", id))
		default:
			log.Warn("UserAuthenticate fail : [%s] %s", errRes.Error.Code, errRes.Error.Desc)
		}
		return ROLE_UNKNOWN, errors.New(fmt.Sprintf("unahtorized for user %s", id))
	}

	return ToRole(res.Role), nil
}
