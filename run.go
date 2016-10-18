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
	"fmt" // 格式化I/O
	"io/ioutil" // I/O效率方法
	"os"	// 操作系统函数
	path "path/filepath" // 实现了兼容各操作系统的文件路径的实用操作函数。别名定义为path
	"runtime" // 提供和go运行时环境的互操作
	"strings" // 操作字符串简单方法
)

var cmdRun = &Command{
	UsageLine: "run [appname] [watchall] [-main=*.go] [-downdoc=true]  [-gendoc=true] [-vendor=true] [-e=folderToExclude]  [-tags=goBuildTags] [-runmode=BEEGO_RUNMODE]",
	Short:     "run the app and start a Web server for development",
	Long: `
Run command will supervise the file system of the beego project using inotify,
it will recompile and restart the app after any modifications.

`,
}

var (
	mainFiles ListOpts
	downdoc   docValue
	gendoc    docValue
	// The flags list of the paths excluded from watching
	excludedPaths strFlags
	// Pass through to -tags arg of "go build"
	buildTags string
	// Application path
	currpath string
	// Application name
	appname string
	// Channel to signal an Exit
	exit chan bool
	// Flag to watch the vendor folder
	vendorWatch bool
	// Current user workspace
	currentGoPath string
	// Current runmode
	runmode string
)

func init() {
	// 绑定方法
	cmdRun.Run = runApp
	// func (f *FlagSet) Var(value Value, name string, usage string)
	// Var方法使用指定的名字、使用信息注册一个flag。
	// 该flag的类型和值由第一个参数表示，该参数应实现了Value接口。
	// 例如，用户可以创建一个flag，可以用Value接口的Set方法将逗号分隔的字符串转化为字符串切片。
	cmdRun.Flag.Var(&mainFiles, "main", "specify main go files")
	cmdRun.Flag.Var(&gendoc, "gendoc", "auto generate the docs")
	cmdRun.Flag.Var(&downdoc, "downdoc", "auto download swagger file when not exist")
	cmdRun.Flag.Var(&excludedPaths, "e", "Excluded paths[].")
	// func (f *FlagSet) Int(name string, value int, usage string) *int
	// Int用指定的名称、默认值、使用信息注册一个int类型flag。返回一个保存了该flag的值的指针。
	cmdRun.Flag.BoolVar(&vendorWatch, "vendor", false, "Watch vendor folder")
	// func (f *FlagSet) StringVar(p *string, name string, value string, usage string)
	// StringVar用指定的名称、默认值、使用信息注册一个string类型flag，并将flag的值保存到p指向的变量。
	cmdRun.Flag.StringVar(&buildTags, "tags", "", "Build tags (https://golang.org/pkg/go/build/)")
	cmdRun.Flag.StringVar(&runmode, "runmode", "", "Set BEEGO_RUNMODE env variable.")
	exit = make(chan bool)
}

func runApp(cmd *Command, args []string) int {
	ShowShortVersionBanner()

	if len(args) == 0 || args[0] == "watchall" {
		// func Getwd() (dir string, err error)
		// Getwd返回一个对应当前工作目录的根路径。
		// 如果当前目录可以经过多条路径抵达（因为硬链接），Getwd会返回其中一个。
		currpath, _ = os.Getwd()
		// SearchGOPATHs ./util.go
		if found, _gopath, _ := SearchGOPATHs(currpath); found {
			// func Base(path string) string
			// Base函数返回路径的最后一个元素。
			// 在提取元素前会求掉末尾的路径分隔符。如果路径是""，会返回"."；
			// 如果路径是只有一个斜杆构成，会返回单个路径分隔符。
			appname = path.Base(currpath)
			currentGoPath = _gopath
		} else {
			// func Sprintf(format string, a ...interface{}) string
			// Sprintf根据format参数生成格式化的字符串并返回该字符串。
			
			// exitPrint ./pack.go
			exitPrint(fmt.Sprintf("Bee does not support non Beego project: %s", currpath))
		}
		ColorLog("[INFO] Using '%s' as 'appname'\n", appname)
	} else {
		// Check if passed Bee application path/name exists in the GOPATH(s)
		if found, _gopath, _path := SearchGOPATHs(args[0]); found {
			currpath = _path
			currentGoPath = _gopath
			appname = path.Base(currpath)
		} else {
			panic(fmt.Sprintf("No Beego application '%s' found in your GOPATH", args[0]))
		}

		ColorLog("[INFO] Using '%s' as 'appname'\n", appname)
		
		//func HasSuffix(s, suffix string) bool
		// 判断s是否有后缀字符串suffix。
		
		// isExist ./util.go
		if strings.HasSuffix(appname, ".go") && isExist(currpath) {
			ColorLog("[WARN] The appname is in conflict with currpath's file, do you want to build appname as %s\n", appname)
			ColorLog("[INFO] Do you want to overwrite it? [yes|no]]  ")
			// askForConfirmation() ./util.go
			// 通过命令行输入 判断用户是否同意
			if !askForConfirmation() {
				return 0
			}
		}
	}
	// Debugf ./util.go
	Debugf("current path:%s\n", currpath)

	if runmode == "prod" || runmode == "dev"{
		os.Setenv("BEEGO_RUNMODE", runmode)
		ColorLog("[INFO] Using '%s' as 'runmode'\n", os.Getenv("BEEGO_RUNMODE"))
	}else if runmode != ""{
		os.Setenv("BEEGO_RUNMODE", runmode)
		ColorLog("[WARN] Using '%s' as 'runmode'\n", os.Getenv("BEEGO_RUNMODE"))
	}else if os.Getenv("BEEGO_RUNMODE") != ""{
		ColorLog("[WARN] Using '%s' as 'runmode'\n", os.Getenv("BEEGO_RUNMODE"))
	}
	// loadConfig() ./conf.go
	// 解析配置
	err := loadConfig()
	if err != nil {
		ColorLog("[ERRO] Fail to parse bee.json[ %s ]\n", err)
	}

	var paths []string
	readAppDirectories(currpath, &paths)

	// Because monitor files has some issues, we watch current directory
	// and ignore non-go files.
	for _, p := range conf.DirStruct.Others {
		paths = append(paths, strings.Replace(p, "$GOPATH", currentGoPath, -1))
	}

	files := []string{}
	for _, arg := range mainFiles {
		if len(arg) > 0 {
			files = append(files, arg)
		}
	}
	if downdoc == "true" {
		// func Stat(name string) (fi FileInfo, err error)
		// Stat返回一个描述name指定的文件对象的FileInfo。
		// 如果指定的文件对象是一个符号链接，返回的FileInfo描述该符号链接指向的文件的信息，本函数会尝试跳转该链接。
		// 如果出错，返回的错误值为*PathError类型。
		
		// func Join(elem ...string) string
		// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加路径分隔符。
		// 结果是经过简化的，所有的空字符串元素会被忽略。
		if _, err := os.Stat(path.Join(currpath, "swagger", "index.html")); err != nil {
			// func IsNotExist(err error) bool
			// 返回一个布尔值说明该错误是否表示一个文件或目录不存在。
			// ErrNotExist和一些系统调用错误会使它返回真。
			if os.IsNotExist(err) {
				downloadFromURL(swaggerlink, "swagger.zip")
				unzipAndDelete("swagger.zip")
			}
		}
	}
	if gendoc == "true" {
		NewWatcher(paths, files, true)
		Autobuild(files, true)
	} else {
		NewWatcher(paths, files, false)
		Autobuild(files, false)
	}

	for {
		select {
		case <-exit:
			runtime.Goexit()
		}
	}
}

func readAppDirectories(directory string, paths *[]string) {
	// func ReadDir(dirname string) ([]os.FileInfo, error)
	// 返回dirname指定的目录的目录信息的有序列表。
	fileInfos, err := ioutil.ReadDir(directory)
	if err != nil {
		return
	}

	useDirectory := false
	for _, fileInfo := range fileInfos {
		// func HasSuffix(s, suffix string) bool
		// 判断s是否有后缀字符串suffix。
		
		//fileInfo.Name() os 文件的名字（不含扩展名）
		if strings.HasSuffix(fileInfo.Name(), "docs") {
			continue
		}
		if strings.HasSuffix(fileInfo.Name(), "swagger") {
			continue
		}

		if !vendorWatch && strings.HasSuffix(fileInfo.Name(), "vendor") {
			continue
		}

		if isExcluded(path.Join(directory, fileInfo.Name())) {
			continue
		}

		if fileInfo.IsDir() == true && fileInfo.Name()[0] != '.' {
			readAppDirectories(directory+"/"+fileInfo.Name(), paths)
			continue
		}

		if useDirectory == true {
			continue
		}

		if path.Ext(fileInfo.Name()) == ".go" {
			*paths = append(*paths, directory)
			useDirectory = true
		}
	}
	return
}

// If a file is excluded
func isExcluded(filePath string) bool {
	for _, p := range excludedPaths {
		// func Abs(path string) (string, error)
		// Abs函数返回path代表的绝对路径，如果path不是绝对路径，会加入当前工作目录以使之成为绝对路径。
		// 因为硬链接的存在，不能保证返回的绝对路径是唯一指向该地址的绝对路径。
		absP, err := path.Abs(p)
		if err != nil {
			ColorLog("[ERROR] Can not get absolute path of [ %s ]\n", p)
			continue
		}
		absFilePath, err := path.Abs(filePath)
		if err != nil {
			ColorLog("[ERROR] Can not get absolute path of [ %s ]\n", filePath)
			break
		}
		// func HasPrefix(s, prefix string) bool
		// 判断s是否有前缀字符串prefix。
		if strings.HasPrefix(absFilePath, absP) {
			ColorLog("[INFO] Excluding from watching [ %s ]\n", filePath)
			return true
		}
	}
	return false
}
