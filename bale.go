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
	"bytes"	// 操作[]byte的常用函数。本包的函数和strings包的函数相当类似。
	"compress/gzip"	// 实现了gzip格式压缩文件的读写
	"fmt"	// 格式化i/o
	"io"	// I/O基本接口
	"os"	// 系统函数
	"path"	// 对斜杠分隔的路径的实用操作函数
	"path/filepath"	// 文件路径函数
	"runtime"	// 环境操作
	"strings"	// 字符串操作
)

var cmdBale = &Command{
	UsageLine: "bale",
	Short:     "packs non-Go files to Go source files",
	Long: `
Bale command compress all the static files in to a single binary file.

This is usefull to not have to carry static files including js, css, images
and views when publishing a project.

auto-generate unpack function to main package then run it during the runtime.
This is mainly used for zealots who are requiring 100% Go code.

`,
}

func init() {
	cmdBale.Run = runBale
}

func runBale(cmd *Command, args []string) int {
	ShowShortVersionBanner()

	err := loadConfig()
	if err != nil {
		ColorLog("[ERRO] Fail to parse bee.json[ %s ]\n", err)
	}
	
	// func RemoveAll(path string) error
	// RemoveAll删除path指定的文件，或目录及它包含的任何下级对象。
	// 它会尝试删除所有东西，除非遇到错误并返回。
	// 如果path指定的对象不存在，RemoveAll会返回nil而不返回错误。
	os.RemoveAll("bale")
	os.Mkdir("bale", os.ModePerm)

	// Pack and compress data.
	for _, p := range conf.Bale.Dirs {
		if !isExist(p) {
			ColorLog("[WARN] Skipped directory( %s )\n", p)
			continue
		}
		ColorLog("[INFO] Packaging directory( %s )\n", p)
		// func Walk(root string, walkFn WalkFunc) error
		// Walk函数会遍历root指定的目录下的文件树，对每一个该文件树中的目录和文件都会调用walkFn，包括root自身。
		// 所有访问文件/目录时遇到的错误都会传递给walkFn过滤。
		// 文件是按词法顺序遍历的，这让输出更漂亮，但也导致处理非常大的目录时效率会降低。
		// Walk函数不会遍历文件树中的符号链接（快捷方式）文件包含的路径。
		filepath.Walk(p, walkFn)
	}

	// Generate auto-uncompress function.
	
	// Buffer是一个实现了读写方法的可变大小的字节缓冲。
	// 本类型的零值是一个空的可用于读写的缓冲。
	buf := new(bytes.Buffer)
	// func (b *Buffer) WriteString(s string) (n int, err error)
	// Write将s的内容写入缓冲中，如必要会增加缓冲容量。返回值n为len(p)，err总是nil。
	// 如果缓冲变得太大，Write会采用错误值ErrTooLarge引发panic。
	
	// func Sprintf(format string, a ...interface{}) string
	// Sprintf根据format参数生成格式化的字符串并返回该字符串。
	
	// func Join(a []string, sep string) string
	// 将一系列字符串连接为一个字符串，之间用sep来分隔。
	buf.WriteString(fmt.Sprintf(BaleHeader, conf.Bale.Import,
		strings.Join(resFiles, "\",\n\t\t\""),
		strings.Join(resFiles, ",\n\t\tbale.R")))
	// func Create(name string) (file *File, err error)
	// Create采用模式0666（任何人都可读写，不可执行）创建一个名为name的文件，如果文件已存在会截断它（为空文件）。
	// 如果成功，返回的文件对象可用于I/O；对应的文件描述符具有O_RDWR模式。
	// 如果出错，错误底层类型是*PathError。
	fw, err := os.Create("bale.go")
	if err != nil {
		ColorLog("[ERRO] Fail to create file[ %s ]\n", err)
		os.Exit(2)
	}
	// func (f *File) Close() error
	// Close关闭文件f，使文件不能用于读写。它返回可能出现的错误。
	defer fw.Close()
	// func (f *File) Write(b []byte) (n int, err error)
	// Write向文件中写入len(b)字节数据。
	// 它返回写入的字节数和可能遇到的任何错误。
	// 如果返回值n!=len(b)，本方法会返回一个非nil的错误。
	_, err = fw.Write(buf.Bytes())
	if err != nil {
		ColorLog("[ERRO] Fail to write data[ %s ]\n", err)
		os.Exit(2)
	}

	ColorLog("[SUCC] Baled resources successfully!\n")
	return 0
}

const (
	BaleHeader = `package main

import(
	"os"
	"strings"
	"path"

	"%s"
)

func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func init() {
	files := []string{
		"%s",
	}

	funcs := []func() []byte{
		bale.R%s,
	}

	for i, f := range funcs {
		fp := getFilePath(files[i])
		if !isExist(fp) {
			saveFile(fp, f())
		}
	}
}

func getFilePath(name string) string {
	name = strings.Replace(name, "_4_", "/", -1)
	name = strings.Replace(name, "_3_", " ", -1)
	name = strings.Replace(name, "_2_", "-", -1)
	name = strings.Replace(name, "_1_", ".", -1)
	name = strings.Replace(name, "_0_", "_", -1)
	return name
}

func saveFile(filePath string, b []byte) (int, error) {
	os.MkdirAll(path.Dir(filePath), os.ModePerm)
	fw, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer fw.Close()
	return fw.Write(b)
}
`
)

var resFiles = make([]string, 0, 10)
// type FileInfo interface {
//     Name() string       // 文件的名字（不含扩展名）
//     Size() int64        // 普通文件返回值表示其大小；其他文件的返回值含义各系统不同
//     Mode() FileMode     // 文件的模式位
//     ModTime() time.Time // 文件的修改时间
//     IsDir() bool        // 等价于Mode().IsDir()
//     Sys() interface{}   // 底层数据来源（可以返回nil）
// }
// FileInfo用来描述一个文件对象。
func walkFn(resPath string, info os.FileInfo, err error) error {
	if info.IsDir() || filterSuffix(resPath) {
		return nil
	}

	// Open resource files.
	fr, err := os.Open(resPath)
	if err != nil {
		ColorLog("[ERRO] Fail to read file[ %s ]\n", err)
		os.Exit(2)
	}

	// Convert path.
	resPath = strings.Replace(resPath, "_", "_0_", -1)
	resPath = strings.Replace(resPath, ".", "_1_", -1)
	resPath = strings.Replace(resPath, "-", "_2_", -1)
	resPath = strings.Replace(resPath, " ", "_3_", -1)
	sep := "/"
	// const GOOS string = theGoos
	// GOOS是可执行程序的目标操作系统（将要在该操作系统的机器上执行）：darwin、freebsd、linux等。
	if runtime.GOOS == "windows" {
		sep = "\\"
	}
	// func Replace(s, old, new string, n int) string
	// 返回将s中前n个不重叠old子串都替换为new的新字符串，如果n<0会替换所有old子串。
	resPath = strings.Replace(resPath, sep, "_4_", -1)

	// Create corresponding Go source files.
	
	// ModePerm FileMode = 0777 
	// 覆盖所有Unix权限位（用于通过&获取类型位）
	
	// func Dir(path string) string
	// Dir返回路径除去最后一个路径元素的部分，即该路径最后一个元素所在的目录。
	// 在使用Split去掉最后一个元素后，会简化路径并去掉末尾的斜杠。
	// 如果路径是空字符串，会返回"."；如果路径由1到多个斜杠后跟0到多个非斜杠字符组成，会返回"/"；其他任何情况下都不会返回以斜杠结尾的路径。
	
	// func MkdirAll(path string, perm FileMode) error
	// MkdirAll使用指定的权限和名称创建一个目录，包括任何必要的上级目录，并返回nil，否则返回错误。
	// 权限位perm会应用在每一个被本函数创建的目录上。
	// 如果path指定了一个已经存在的目录，MkdirAll不做任何操作并返回nil。
	os.MkdirAll(path.Dir(resPath), os.ModePerm)
	fw, err := os.Create("bale/" + resPath + ".go")
	if err != nil {
		ColorLog("[ERRO] Fail to create file[ %s ]\n", err)
		os.Exit(2)
	}
	defer fw.Close()

	// Write header.
	
	// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error)
	// Fprintf根据format参数生成格式化的字符串并写入w。返回写入的字节数和遇到的任何错误。
	fmt.Fprintf(fw, Header, resPath)

	// Copy and compress data.
	
	// 
	gz := gzip.NewWriter(&ByteWriter{Writer: fw})
	io.Copy(gz, fr)
	gz.Close()

	// Write footer.
	fmt.Fprint(fw, Footer)

	resFiles = append(resFiles, resPath)
	return nil
}

func filterSuffix(name string) bool {
	for _, s := range conf.Bale.IngExt {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

const (
	Header = `package bale

import(
	"bytes"
	"compress/gzip"
	"io"
)

func R%s() []byte {
	gz, err := gzip.NewReader(bytes.NewBuffer([]byte{`
	Footer = `
	}))

	if err != nil {
		panic("Unpack resources failed: " + err.Error())
	}

	var b bytes.Buffer
	io.Copy(&b, gz)
	gz.Close()

	return b.Bytes()
}`
)

var newline = []byte{'\n'}

type ByteWriter struct {
	io.Writer
	c int
}

func (w *ByteWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	for n = range p {
		if w.c%12 == 0 {
			w.Writer.Write(newline)
			w.c = 0
		}

		fmt.Fprintf(w.Writer, "0x%02x,", p[n])
		w.c++
	}

	n++

	return
}
