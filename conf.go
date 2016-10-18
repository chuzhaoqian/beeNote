// Copyright 2013 bee authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package main

import (
	"encoding/json"	// json 编码包
	"io/ioutil"	// 有效的i/o方法
	"os"	//系统函数

	"gopkg.in/yaml.v2"	// 实现Go语言的YAML的支持。
)

const ConfVer = 0

var defaultConf = `{
	"version": 0,
	"gopm": {
		"enable": false,
		"install": false
	},
	"go_install": false,
	"watch_ext": [],
	"dir_structure": {
		"watch_all": false,
		"controllers": "",
		"models": "",
		"others": []
	},
	"cmd_args": [],
	"envs": [],
	"database": {
		"driver": "mysql"
	}
}
`
var conf struct {
	Version int
	// gopm support
	Gopm struct {
		Enable  bool
		Install bool
	}
	// Indicates whether execute "go install" before "go build".
	GoInstall bool     `json:"go_install" yaml:"go_install"`
	WatchExt  []string `json:"watch_ext" yaml:"watch_ext"`
	DirStruct struct {
		WatchAll    bool `json:"watch_all" yaml:"watch_all"`
		Controllers string
		Models      string
		Others      []string // Other directories.
	} `json:"dir_structure" yaml:"dir_structure"`
	CmdArgs []string `json:"cmd_args" yaml:"cmd_args"`
	Envs    []string
	Bale    struct {
		Import string
		Dirs   []string
		IngExt []string `json:"ignore_ext" yaml:"ignore_ext"`
	}
	Database struct {
		Driver string
		Conn   string
	}
}

// loadConfig loads customized configuration.
func loadConfig() error {
	foundConf := false
	// func Open(name string) (file *File, err error)
	// Open打开一个文件用于读取。
	// 如果操作成功，返回的文件对象的方法可用于读取数据；
	// 对应的文件描述符具有O_RDONLY模式。如果出错，错误底层类型是*PathError。
	f, err := os.Open("bee.json")
	if err == nil {
		// func (f *File) Close() error
		// Close关闭文件f，使文件不能用于读写。它返回可能出现的错误。
		defer f.Close()
		ColorLog("[INFO] Detected bee.json\n")
		// func NewDecoder(r io.Reader) *Decoder
		// NewDecoder创建一个从r读取并解码json对象的*Decoder，解码器有自己的缓冲，并可能超前读取部分json数据。
		d := json.NewDecoder(f)
		// func (dec *Decoder) Decode(v interface{}) error
		// Decode从输入流读取下一个json编码值并保存在v指向的值里，参见Unmarshal函数的文档获取细节信息。
		err = d.Decode(&conf)
		if err != nil {
			return err
		}
		foundConf = true
	}
	//func ReadFile(filename string) ([]byte, error)
	//ReadFile 从filename指定的文件中读取数据并返回文件的内容。
	//成功的调用返回的err为nil而非EOF。因为本函数定义为读取整个文件，它不会将读取返回的EOF视为应报告的错误。
	byml, erryml := ioutil.ReadFile("Beefile")
	if erryml == nil {
		ColorLog("[INFO] Detected Beefile\n")
		err = yaml.Unmarshal(byml, &conf)
		if err != nil {
			return err
		}
		foundConf = true
	}
	if !foundConf {
		// Use default.
		// func Unmarshal(data []byte, v interface{}) error
		// Unmarshal函数解析json编码的数据并将结果存入v指向的值。
		err = json.Unmarshal([]byte(defaultConf), &conf)
		if err != nil {
			return err
		}
	}
	// Check format version.
	if conf.Version != ConfVer {
		ColorLog("[WARN] Your bee.json is out-of-date, please update!\n")
		ColorLog("[HINT] Compare bee.json under bee source code path and yours\n")
	}

	// Set variables.
	if len(conf.DirStruct.Controllers) == 0 {
		conf.DirStruct.Controllers = "controllers"
	}
	if len(conf.DirStruct.Models) == 0 {
		conf.DirStruct.Models = "models"
	}

	// Append watch exts.
	watchExts = append(watchExts, conf.WatchExt...)
	return nil
}
