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
// @date 2017. 2. 22. AM 11:27
//

package domain

import (
	"strings"
	"time"
)

const (
	TIME_YYYYMMDDHHMMSS = "2006-01-02 15:04:05"
	JUNO_STATUS_ALIVE   = "A"
	JUNO_STATUS_DEAD    = "D"
)

type JunoStatus string

type JunoRepository interface {
	FindAll() *JunoSummary
	FindGroup(groupName string) *JunoGroup
	FindByPoint(point PackagePoint) *JunoPackage
	FindByAddress(address string) *JunoPackage
	FindByEndpoint(endpoint string) *JunoPackage
	Save(data JunoRegistration)
	Delete(endpoint string)
	SaveAll()
}

type Juno struct {
	Group    string `json:"group"`
	Host     string `json:"host"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

type JunoPackage struct {
	Endpoint   string       `json:"endpoint"`
	Host       string       `json:"host"`
	Name       string       `json:"name"`
	RegistDate interface{}  `json:"regist_date"`
	Status     string       `json:"status"`
	Platform   PlatformInfo `json:"platform"`
}

func (jp *JunoPackage) Format(location *time.Location) JunoPackage {
	juno := JunoPackage{}
	juno.Endpoint = jp.Endpoint
	juno.Host = jp.Host
	juno.Name = jp.Name
	juno.Status = jp.Status
	if juno.Status == JUNO_STATUS_DEAD {
		juno.RegistDate = "-"
		return juno
	}

	var timeMillis int64
	switch jp.RegistDate.(type) {
	case string:
		juno.RegistDate = jp.RegistDate
		return juno
	case float64:
		timeMillis = int64(jp.RegistDate.(float64))
	case int64:
		timeMillis = jp.RegistDate.(int64)
	}
	juno.RegistDate = time.Unix(timeMillis, 0).In(location).Format(TIME_YYYYMMDDHHMMSS)
	juno.Platform = jp.Platform
	return juno
}

type JunoRegistration struct {
	Group string
	JunoPackage
}

func (jr *JunoRegistration) AsJunoPackage() JunoPackage {
	return JunoPackage{Endpoint: jr.Endpoint, Host: jr.Host, Name: jr.Name, RegistDate: jr.RegistDate, Status: jr.Status}
}

type JunoEndpointResponse struct {
	Endpoint string        `json:"endpoint,omitempty"`
	System   SystemMessage `json:"system"`
}

type JunoSummary struct {
	GroupCount   int         `json:"group_count"`
	HostCount    int         `json:"host_count"`
	PackageCount int         `json:"package_count"`
	Groups       []JunoGroup `json:"groups,omitempty"`
}

func (js *JunoSummary) UpdateHostCount() {
	hostMap := make(map[string]int)
	for _, g := range js.Groups {
		for _, p := range g.Packages {
			hostMap[strings.ToLower(p.Host)]++
		}
	}

	total := 0
	for _, v := range hostMap {
		total = total + v
	}
	js.HostCount = total
}

func (js *JunoSummary) FindGroup(groupName string) *JunoGroup {
	if js.Groups == nil {
		return nil
	}

	compName := strings.ToLower(groupName)

	for i := 0; i < len(js.Groups); i++ {
		if js.Groups[i].Name == compName {
			return &js.Groups[i]
		}
	}

	return nil
}

func (js *JunoSummary) FindByPoint(point PackagePoint) *JunoPackage {
	compName := strings.ToLower(point.Name)
	compHost := strings.ToLower(point.Host)

	for i := 0; i < len(js.Groups); i++ {
		for j := 0; j < len(js.Groups[i].Packages); j++ {
			if strings.ToLower(js.Groups[i].Packages[j].Host) ==
				compHost && strings.ToLower(js.Groups[i].Packages[j].Name) == compName {
				return &js.Groups[i].Packages[j]
			}
		}
	}

	return nil
}

func (js *JunoSummary) FindByEndpoint(endpoint string) *JunoPackage {
	for i := 0; i < len(js.Groups); i++ {
		for j := 0; j < len(js.Groups[i].Packages); j++ {
			if js.Groups[i].Packages[j].Endpoint == endpoint {
				return &js.Groups[i].Packages[j]
			}
		}
	}

	return nil
}

func (js *JunoSummary) DeleteGroup(index int) {
	js.Groups = append(js.Groups[:index], js.Groups[index+1:]...)
}

func (js *JunoSummary) AppendGroup(group *JunoGroup) {
	if js.Groups == nil {
		js.Groups = make([]JunoGroup, 0)
	}

	js.Groups = append(js.Groups, *group)
}

type JunoGroup struct {
	Name     string        `json:"group_name"`
	Packages []JunoPackage `json:"deploy,omitempty"`
}

func (jg *JunoGroup) Append(pack JunoPackage) {
	jg.Packages = append(jg.Packages, pack)
}

func (jg *JunoGroup) Delete(index int) {
	jg.Packages = append(jg.Packages[:index], jg.Packages[index+1:]...)
}

func (jg *JunoGroup) Clone(location *time.Location) JunoGroup {
	g := JunoGroup{}
	g.Name = jg.Name
	g.Packages = make([]JunoPackage, 0)
	if len(jg.Packages) > 0 {
		for _, v := range jg.Packages {
			g.Packages = append(g.Packages, v.Format(location))
		}
	}
	return g
}
