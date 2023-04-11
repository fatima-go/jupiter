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
// @date 2017. 3. 15. PM 1:56
//

package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"throosea.com/fatima"
	"throosea.com/fatima/lib"
	"throosea.com/jupiter/domain"
	"throosea.com/jupiter/web"
	"throosea.com/log"
)

type DeployRequest struct {
	filename      string `json:"file_name"`
	group         string
	pack          string
	localpath     string
	clientAddress string
	when          string `json:"when"`
}

func (d DeployRequest) removeLocalFile() {
	if len(d.localpath) > 0 {
		os.Remove(d.localpath)
	}
}

func (interactor *DomainInteractor) DeployPackage(mr *multipart.Reader, clientAddress string, token string) (string, error) {
	req, err := buildDeployRequest(interactor.fatimaRuntime.GetEnv(), mr)
	if err != nil {
		return "", err
	}

	req.clientAddress = clientAddress
	defer req.removeLocalFile()

	// validate
	if len(req.filename) == 0 || !strings.HasSuffix(req.filename, "far") {
		return "", fmt.Errorf("invalid filename : %s", req.filename)
	}

	// get target juno list
	var endpointList []string
	endpointList, err = getEndpointList(req, interactor.JunoRepository)
	endpointLen := len(endpointList)
	if endpointLen == 0 {
		return "", errors.New("not found endpoint")
	}

	for _, e := range endpointList {
		log.Debug("endpoint : %s", e)
	}

	// send to juno
	stat, _ := os.Stat(req.localpath)
	result := fmt.Sprintf("far name : %s (%d bytes). target : %d juno enqueued", req.filename, stat.Size(), len(endpointList))
	log.Info(result)

	cyBarrier := lib.NewCyclicBarrier(endpointLen, func() { log.Info("%s 디플로이 완료", req.filename) })
	for _, v := range endpointList {
		t := v
		cyBarrier.Dispatch(func() {
			e := writeDeployRequestToJuno(req, t, token)
			if e != nil {
				log.Warn("deploy to juno is fail : %s", e.Error())
			}
		})
	}
	cyBarrier.Wait()

	runtime.GC()

	return result, nil
}

// e.g) Content-Disposition: form-data; name="data"; filename="data"
func buildContentDispositionMap(source string) map[string]string {
	var ss []string

	ss = strings.Split(source, ";")
	m := make(map[string]string)
	for _, pair := range ss {
		z := strings.Split(pair, "=")
		if len(z) != 2 {
			continue
		}
		k := strings.Trim(z[0], " ")
		v := strings.Trim(z[1], " ")
		m[k] = v
	}

	return m
}

func buildDeployRequest(env fatima.FatimaEnv, mr *multipart.Reader) (*DeployRequest, error) {
	r := DeployRequest{when: "now"}

	completeCount := 0
	tmpFile := env.GetFolderGuide().CreateTmpFilePath()
	//defer os.Remove(tmpFile)
	log.Debug("tmp path : %s", tmpFile)

	for {
		p, err := mr.NextPart()
		//defer p.Close()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fail to parse multipart data : %s", err.Error())
		}

		s := p.Header.Get("Content-Disposition")
		log.Trace("content disposition : %s", s)
		if len(s) == 0 {
			p.Close()
			return nil, fmt.Errorf("invalid content-disposition value")
		}

		m := buildContentDispositionMap(s)
		name := m["name"]
		if len(name) == 0 {
			p.Close()
			return nil, fmt.Errorf("invalid content-disposition name value")
		}

		slurp, err := ioutil.ReadAll(p)
		name = cutQuatation(name)
		log.Trace("name value : %s", name)
		if name == "far" {
			// form-data; name="far"; filename="example.far"
			err := ioutil.WriteFile(filepath.Join(tmpFile), slurp, 0644)
			if err != nil {
				p.Close()
				return nil, fmt.Errorf("fail to save file to local : %s", err.Error())
			}
			//o := m["filename"]
			//r.filename = cutQuatation(o)
			r.localpath = tmpFile
			completeCount = completeCount + 1
		} else if name == "json" {
			// form-data; name="json"; filename="json"
			var items map[string]string
			err = json.Unmarshal(slurp, &items)
			if err != nil {
				p.Close()
				return nil, fmt.Errorf("fail to unmarshal json data : %s", err.Error())
			}
			r.group = items["group"]
			r.pack = items["package"]
			r.filename = items["file"]
			if len(r.filename) > 0 {
				lastIndex := strings.LastIndex(r.filename, "/")
				if lastIndex >= 0 {
					r.filename = r.filename[lastIndex+1:]
				}
			}

			completeCount = completeCount + 1
		}

		if completeCount >= 2 {
			p.Close()
			break
		}
	}

	return &r, nil
}

// get target juno list
func getEndpointList(req *DeployRequest, repo domain.JunoRepository) ([]string, error) {
	endpointList := make([]string, 0)
	if len(req.group) > 0 {
		g := repo.FindGroup(req.group)
		if g == nil {
			return nil, fmt.Errorf("there are no endpoint for group %s", req.group)
		}
		for _, p := range g.Packages {
			endpointList = append(endpointList, p.Endpoint)
		}
	} else if len(req.pack) > 0 {
		point := domain.NewPackagePoint(req.pack)
		pack := repo.FindByPoint(point)
		if pack == nil {
			return nil, fmt.Errorf("not found endpoint for package %s", pack)
		}
		endpointList = append(endpointList, pack.Endpoint)
	} else {
		pack := repo.FindByAddress(req.clientAddress)
		if pack == nil {
			return nil, fmt.Errorf("not found endpoint")
		}
		endpointList = append(endpointList, pack.Endpoint)
	}
	return endpointList, nil
}

func cutQuatation(value string) string {
	if len(value) < 2 {
		return value
	}

	if value[0] == '\'' || value[0] == '"' {
		value = value[1:]
	}

	i := len(value)
	if i > 0 {
		if value[i-1] == '\'' || value[i-1] == '"' {
			value = value[:i-1]
		}
	}

	return value
}

func writeDeployRequestToJuno(req *DeployRequest, endpoint string, token string) error {
	httpClient := web.NewHttpClient(nil)
	httpClient.SetToken(token)

	boundary := lib.RandomAlphanumeric(32)
	httpClient.SetContentType(fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	startBoundary := "--" + boundary
	endBoundary := fmt.Sprintf("\r\n--%s--", boundary)

	var buff bytes.Buffer
	writeProlog(req, &buff, startBoundary)
	// open file handle
	fh, _ := os.Open(req.localpath)
	defer fh.Close()
	io.Copy(&buff, fh)
	buff.WriteString(endBoundary)

	/*
		String startBoundary = "--" + boundary;
			byte[] endBoundary = new String("\r\n--" + boundary + "--")
					.getBytes(Property.HTTP_CHARACTER_SET);

			// set headers
			con.setRequestProperty("Content-Type", "multipart/form-data; boundary="
					+ boundary);
	*/
	_, err := httpClient.PostWithBuffer(buildRestUrl(endpoint, "/deploy/v1"), &buff)
	if err != nil {
		return err
	}

	return nil
}

func writeProlog(req *DeployRequest, buff *bytes.Buffer, startBoundary string) {
	/*
	StringBuilder prologue = new StringBuilder();

	--dbc9bcd2ae2f4bb98db290b8d949b170
	Content-Disposition: form-data; name="json"; filename="data"

	{"when": "now"}
	--dbc9bcd2ae2f4bb98db290b8d949b170
	Content-Disposition: form-data; name="far"; filename="ps"
	*/
	buff.WriteString(startBoundary)
	buff.WriteString("\r\n")
	buff.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"json\""))
	buff.WriteString("\r\n\r\n")
	d, _ := json.Marshal(req)
	buff.Write(d)
	buff.WriteString("\r\n")
	buff.WriteString(startBoundary)
	buff.WriteString("\r\n")
	buff.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"far\""))
	buff.WriteString("\r\n\r\n")
}
