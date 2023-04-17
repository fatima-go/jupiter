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
// @date 2017. 2. 23. AM 9:31
//

package infra

import (
	"encoding/json"
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/domain"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	JUNO_DEPLOY_DATA_FILE = "juno.json"
)

func NewFileJunoRepository(fatimaRuntime fatima.FatimaRuntime) domain.JunoRepository {
	repo := new(FileJunoRepository)

	repo.junoFilePath = filepath.Join(
		fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(),
		JUNO_DEPLOY_DATA_FILE)

	summary, ok := repo.load()
	if !ok {
		summary = &domain.JunoSummary{GroupCount: 0, HostCount: 0, PackageCount: 0}
		summary.Groups = make([]domain.JunoGroup, 0)
		data, err := json.Marshal(summary)
		if err != nil {
			panic(fmt.Sprintf("fail to build default juno summary : %s", err.Error()))
		}
		ioutil.WriteFile(repo.junoFilePath, data, 0644)
		log.Info("created default juno data file")
	}

	repo.summary = summary
	return repo
}

type FileJunoRepository struct {
	junoFilePath string
	summary      *domain.JunoSummary
}

func (handler *FileJunoRepository) load() (*domain.JunoSummary, bool) {
	data, err := ioutil.ReadFile(handler.junoFilePath)
	if err != nil {
		log.Warn("read file fail : %s", err.Error())
		return nil, false
	}

	var summary domain.JunoSummary
	err = json.Unmarshal(data, &summary)
	if err != nil {
		log.Warn("json fail : %s", err.Error())
		return nil, false
	}

	return &summary, true
}

func (handler *FileJunoRepository) sync() {
	if handler.summary == nil {
		return
	}

	data, err := json.Marshal(handler.summary)
	if err != nil {
		panic(fmt.Sprintf("fail to build default juno summary : %s", err.Error()))
	}
	os.WriteFile(handler.junoFilePath, data, 0644)
	log.Debug("juno summary sync to json file")
}

func (handler *FileJunoRepository) FindAll() *domain.JunoSummary {
	summary, ok := handler.load()
	if !ok {
		return handler.summary
	}
	handler.summary = summary
	return handler.summary
}

func (handler *FileJunoRepository) FindByPoint(point domain.PackagePoint) *domain.JunoPackage {
	summary := handler.summary

	if summary == nil {
		return nil
	}

	if summary.PackageCount == 0 {
		return nil
	}

	if len(point.Host) < 1 {
		if summary.PackageCount == 1 {
			if len(summary.Groups) == 0 || len(summary.Groups[0].Packages) == 0 {
				log.Warn("invalid juno summary. JunoPackageCount is 1 but there are no juno data")
				return nil
			}
			return &summary.Groups[0].Packages[0]
		}
		return nil
	}

	return summary.FindByPoint(point)
}

func (handler *FileJunoRepository) FindByAddress(address string) *domain.JunoPackage {
	summary := handler.summary

	if summary == nil {
		return nil
	}

	if summary.PackageCount == 0 {
		return nil
	} else if summary.PackageCount == 1 {
		return &summary.Groups[0].Packages[0]
	}

	ip := ExtractIpAddress(address)
	for _, g := range summary.Groups {
		for _, p := range g.Packages {
			if strings.HasPrefix(p.Endpoint, "http://") {
				comp := p.Endpoint[7:] // e.g comp => 10.180.37.134:9180/QYNMdOrq/
				delim := strings.Index(comp, ":")
				if delim < 0 {
					if strings.HasPrefix(comp, ip) {
						return &p
					}
				} else {
					if comp[:delim] == ip {
						return &p
					}
				}
			} else if strings.HasPrefix(p.Endpoint, ip) {
				return &p
			}

		}
	}

	return nil
}

func (handler *FileJunoRepository) FindGroup(groupName string) *domain.JunoGroup {
	summary := handler.summary

	comp := strings.ToLower(groupName)
	for i := 0; i < len(summary.Groups); i++ {
		if comp == strings.ToLower(summary.Groups[i].Name) {
			return &summary.Groups[i]
		}
	}

	return nil
}

func (handler *FileJunoRepository) FindByEndpoint(endpoint string) *domain.JunoPackage {
	summary := handler.summary

	if summary == nil || len(endpoint) < 1 {
		return nil
	}

	if summary.PackageCount == 0 {
		return nil
	}

	for i := 0; i < len(summary.Groups); i++ {
		for j := 0; j < len(summary.Groups[i].Packages); j++ {
			if summary.Groups[i].Packages[j].Endpoint == endpoint {
				return &summary.Groups[i].Packages[j]
			}
		}
	}

	return nil
}

func (handler *FileJunoRepository) Save(data domain.JunoRegistration) {
	summary := handler.summary
	compGroup := strings.ToLower(data.Group)
	for i := 0; i < len(summary.Groups); i++ {
		if strings.ToLower(summary.Groups[i].Name) == compGroup {
			summary.Groups[i].Append(data.AsJunoPackage())
			summary.PackageCount = summary.PackageCount + 1
			summary.UpdateHostCount()
			handler.sync()
			return
		}
	}

	group := domain.JunoGroup{Name: data.Group}
	group.Packages = make([]domain.JunoPackage, 1)
	group.Packages[0] = data.AsJunoPackage()
	summary.Groups = append(summary.Groups, group)
	summary.GroupCount = summary.GroupCount + 1
	summary.PackageCount = summary.PackageCount + 1
	summary.UpdateHostCount()
	handler.sync()
	return
}

func (handler *FileJunoRepository) Delete(endpoint string) {
	if handler.summary == nil {
		return
	}

	found := -1
	for i := 0; i < len(handler.summary.Groups); i++ {
		for i, p := range handler.summary.Groups[i].Packages {
			if p.Endpoint == endpoint {
				found = i
				break
			}
		}
		if found >= 0 {
			handler.summary.Groups[i].Delete(found)
			if len(handler.summary.Groups[i].Packages) == 0 {
				handler.summary.DeleteGroup(i)
				handler.summary.GroupCount = len(handler.summary.Groups)
			}
			handler.summary.PackageCount = handler.summary.PackageCount - 1
			handler.summary.UpdateHostCount()
			handler.sync()
			break
		}
	}
}

func (handler *FileJunoRepository) SaveAll() {
	handler.sync()
}
