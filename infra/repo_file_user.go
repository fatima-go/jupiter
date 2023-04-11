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
// @date 2017. 2. 23. AM 9:30
//

package infra

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"throosea.com/fatima"
	"throosea.com/jupiter/domain"
	"throosea.com/log"
)

const (
	USER_DATA_FILE = "user_data.xml"
)

type GatewayUser struct {
	Id       string `xml:"id,attr"`
	Password string `xml:"passwd,attr"`
	Role     string `xml:"role,attr"`
}

type GatewayUserData struct {
	XMLName xml.Name      `xml:"gateway_user"`
	Users   []GatewayUser `xml:"user"`
}

func NewFileUserRepository(fatimaRuntime fatima.FatimaRuntime) domain.UserRepository {
	repo := new(FileUserRepository)

	repo.xmlUserData = loadXmlUserData(fatimaRuntime.GetEnv().GetFolderGuide())
	repo.build()
	return repo
}

func loadXmlUserData(folderGuide fatima.FolderGuide) GatewayUserData {
	filePath := filepath.Join(folderGuide.GetDataFolder(), USER_DATA_FILE)
	log.Debug("using xml : %s", filePath)
	data, err := ioutil.ReadFile(filePath)
	var xmlUserData GatewayUserData
	if err != nil {
		if os.IsNotExist(err) {
			xmlUserData = GatewayUserData{}
			xmlUserData.Users = make([]GatewayUser, 1)
			xmlUserData.Users[0] = GatewayUser{Id: "admin", Password: "admin", Role: "OPERATOR"}
			data, err = xml.Marshal(&xmlUserData)
			if err != nil {
				panic(fmt.Sprintf("fail to create default gateway user file : %s", err.Error()))
			}
			ioutil.WriteFile(filePath, data, 0644)
			log.Info("created default gateway user xml file")
		} else {
			panic(fmt.Sprintf("fail to load gateway user file : %s", err.Error()))
		}
	} else {
		err = xml.Unmarshal([]byte(data), &xmlUserData)
		if err != nil {
			panic(fmt.Sprintf("fail to load user data xml file : %s", err))
		}
	}

	return xmlUserData
}

type FileUserRepository struct {
	xmlUserData GatewayUserData
	userDataMap map[string]GatewayUser
}

func (handler *FileUserRepository) build() {
	userDataMap := make(map[string]GatewayUser)
	for _, v := range handler.xmlUserData.Users {
		userDataMap[v.Id] = v
	}

	handler.userDataMap = userDataMap
}

func (handler *FileUserRepository) Save(user domain.User) {
	log.Warn("not yet implemented")
}

func (handler *FileUserRepository) FindById(id string) *domain.User {
	u, ok := handler.userDataMap[id]
	if !ok {
		return nil
	}

	user := domain.User{}
	user.Id = u.Id
	user.Password = u.Password
	user.Role = domain.ToRole(u.Role)
	return &user
}

func (handler *FileUserRepository) Delete(id string) {
	log.Warn("not yet implemented")
}

func (handler *FileUserRepository) Exists(id string) bool {
	_, ok := handler.userDataMap[id]
	return ok
}

func (handler *FileUserRepository) Count() int {
	return len(handler.userDataMap)
}
