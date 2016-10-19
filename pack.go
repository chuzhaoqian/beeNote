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
	"archive/tar" // tar包实现了tar格式压缩文件的存取
	"archive/zip" // zip包提供了zip档案文件的读写服务。不支持跨硬盘的压缩。
	"compress/gzip" // gzip包实现了gzip格式压缩文件的读写
	"flag" // 命令行参数的解析
	"fmt"	// 格式化i/o
	"io"	// I/O原语的基本接口
	"io/ioutil" // 有效率的i/o函数
	"os"	// 操作系统函数
	"os/exec" // 行外部命令
	path "path/filepath" // 文件路径的实用操作函数
	"regexp" // 正则
	"runtime" // 和go运行时环境的互操作
	"sort" // 排序切片和用户自定义数据集的函数
	"strconv" // 基本数据类型和其字符串表示的相互转换
	"strings" // 操作字符的简单函数
	"syscall"	//封装系统调用,包含底层操作系统原语。
			//细节取决于底层系统，默认情况下，godoc将显示当前系统的系统调用的文件。
	"time"	// 时间的显示和测量用的函数。日历的计算采用的是公历。
)

var cmdPack = &Command{
	CustomFlags: true,
	UsageLine:   "pack",
	Short:       "Compress a beego project into a single file",
	Long: `
Pack is used to compress a beego project into a single file.
This eases the deployment by extracting the zip file to a server.

-p            app path (default is the current path).
-b            build specify platform app (default: true).
-ba           additional args of go build
-be=[]        additional ENV Variables of go build. eg: GOARCH=arm
-o            compressed file output dir. default use current path
-f=""         format: tar.gz, zip (default: tar.gz)
-exp=""       relpath exclude prefix (default: .). use : as separator
-exs=""       relpath exclude suffix (default: .go:.DS_Store:.tmp). use : as separator
              all path use : as separator
-exr=[]       file/directory name exclude by Regexp (default: ^).
-fs=false     follow symlink (default: false).
-ss=false     skip symlink (default: false)
              default embed symlink into compressed file
-v=false      verbose
`,
}

var (
	appPath   string
	excludeP  string
	excludeS  string
	outputP   string
	excludeR  ListOpts
	fsym      bool
	ssym      bool
	build     bool
	buildArgs string
	buildEnvs ListOpts
	verbose   bool
	format    string
	w         io.Writer
)

type ListOpts []string

// 串联数据 返回字符串
func (opts *ListOpts) String() string {
	//func Sprint(a ...interface{}) string
	//Sprint采用默认格式将其参数格式化，串联所有输出生成并返回一个字符串。
	//如果两个相邻的参数都不是字符串，会在它们的输出之间添加空格。
	return fmt.Sprint(*opts)
}

// 追加元素
func (opts *ListOpts) Set(value string) error {
	*opts = append(*opts, value)
	return nil
}

func init() {
	// func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet
	// NewFlagSet创建一个新的、名为name，采用errorHandling为错误处理策略的FlagSet。
	
	// type ErrorHandling int
	// ErrorHandling定义如何处理flag解析错误。
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	// func (f *FlagSet) StringVar(p *string, name string, value string, usage string)
	// StringVar用指定的名称、默认值、使用信息注册一个string类型flag，并将flag的值保存到p指向的变量。
	// 要打包的路径 默认是当前目录
	fs.StringVar(&appPath, "p", "", "app path. default is current path")
	// func BoolVar(p *bool, name string, value bool, usage string)
	// BoolVar用指定的名称、默认值、使用信息注册一个bool类型flag，并将flag的值保存到p指向的变量。
	fs.BoolVar(&build, "b", true, "build specify platform app")
	fs.StringVar(&buildArgs, "ba", "", "additional args of go build")
	// func (f *FlagSet) Var(value Value, name string, usage string)
	// Var方法使用指定的名字、使用信息注册一个flag。
	// 该flag的类型和值由第一个参数表示，该参数应实现了Value接口。
	// 例如，用户可以创建一个flag，可以用Value接口的Set方法将逗号分隔的字符串转化为字符串切片。
	fs.Var(&buildEnvs, "be", "additional ENV Variables of go build. eg: GOARCH=arm")
	fs.StringVar(&outputP, "o", "", "compressed file output dir. default use current path")
	fs.StringVar(&format, "f", "tar.gz", "format. [ tar.gz / zip ]")
	fs.StringVar(&excludeP, "exp", ".", "path exclude prefix. use : as separator")
	fs.StringVar(&excludeS, "exs", ".go:.DS_Store:.tmp", "path exclude suffix. use : as separator")
	fs.Var(&excludeR, "exr", "filename exclude by Regexp")
	fs.BoolVar(&fsym, "fs", false, "follow symlink")
	fs.BoolVar(&ssym, "ss", false, "skip symlink")
	fs.BoolVar(&verbose, "v", false, "verbose")
	cmdPack.Flag = *fs
	cmdPack.Run = packApp
	// ./color.go
	w = NewColorWriter(os.Stdout)
}

func exitPrint(con string) {
	// func Fprintln(w io.Writer, a ...interface{}) (n int, err error)
	// Fprintln采用默认格式将其参数格式化并写入w。
	// 总是会在相邻参数的输出之间添加空格并在输出结束后添加换行符。
	// 返回写入的字节数和遇到的任何错误。
	fmt.Fprintln(os.Stderr, con)
	os.Exit(2)
}

type walker interface {
	isExclude(string) bool
	isEmpty(string) bool
	relName(string) string
	virPath(string) string
	compress(string, string, os.FileInfo) (bool, error)
	walkRoot(string) error
}

type byName []os.FileInfo

// 这一组方法 是排序使用的 sort
func (f byName) Len() int           { return len(f) }
func (f byName) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f byName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

type walkFileTree struct {
	wak           walker
	prefix        string
	excludePrefix []string
	excludeRegexp []*regexp.Regexp	//Regexp代表一个编译好的正则表达式。Regexp可以被多线程安全地同时使用。
	excludeSuffix []string
	allfiles      map[string]bool
}

func (wft *walkFileTree) setPrefix(prefix string) {
	wft.prefix = prefix
}

func (wft *walkFileTree) isExclude(fPath string) bool {
	if fPath == "" {
		return true
	}

	for _, prefix := range wft.excludePrefix {
		// func HasPrefix(s, prefix string) bool
		// 判断s是否有前缀字符串prefix。
		if strings.HasPrefix(fPath, prefix) {
			return true
		}
	}
	for _, suffix := range wft.excludeSuffix {
		// func HasSuffix(s, suffix string) bool
		// 判断s是否有后缀字符串suffix。
		if strings.HasSuffix(fPath, suffix) {
			return true
		}
	}
	return false
}

func (wft *walkFileTree) isExcludeName(name string) bool {
	for _, r := range wft.excludeRegexp {
		// func (re *Regexp) MatchString(s string) bool
		// 检查s中是否存在匹配pattern的子序列。
		if r.MatchString(name) {
			return true
		}
	}

	return false
}

func (wft *walkFileTree) isEmpty(fpath string) bool {
	// func Open(name string) (file *File, err error)
	// Open打开一个文件用于读取。
	// 如果操作成功，返回的文件对象的方法可用于读取数据；
	// 对应的文件描述符具有O_RDONLY模式。
	// 如果出错，错误底层类型是*PathError。
	fh, _ := os.Open(fpath)
	// func (f *File) Close() error
	// Close关闭文件f，使文件不能用于读写。它返回可能出现的错误。
	defer fh.Close()
	// func (f *File) Readdir(n int) (fi []FileInfo, err error)
	// Readdir读取目录f的内容，返回一个有n个成员的[]FileInfo，这些FileInfo是被Lstat返回的，采用目录顺序。
	// 对本函数的下一次调用会返回上一次调用剩余未读取的内容的信息。
	// 如果n>0，Readdir函数会返回一个最多n个成员的切片。
	// 这时，如果Readdir返回一个空切片，它会返回一个非nil的错误说明原因。
	// 如果到达了目录f的结尾，返回值err会是io.EOF。
	// 如果n<=0，Readdir函数返回目录中剩余所有文件对象的FileInfo构成的切片。
	// 此时，如果Readdir调用成功（读取所有内容直到结尾），它会返回该切片和nil的错误值。
	// 如果在到达结尾前遇到错误，会返回之前成功读取的FileInfo构成的切片和该错误。
	infos, _ := fh.Readdir(-1)
	for _, fi := range infos {
		// type FileInfo
		// Name() string       
		// 文件的名字（不含扩展名）
		fn := fi.Name()
		
		// func Join(elem ...string) string
		// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加路径分隔符。
		// 结果是经过简化的，所有的空字符串元素会被忽略。
		fp := path.Join(fpath, fn)
		if wft.isExclude(wft.virPath(fp)) {
			continue
		}
		if wft.isExcludeName(fn) {
			continue
		}
		// ModeSymlink
		// L: 符号链接（不是快捷方式文件）
		if fi.Mode()&os.ModeSymlink > 0 {
			continue
		}
		
		// type FileInfo
		// IsDir() bool      
		// 等价于Mode().IsDir()
		if fi.IsDir() && wft.isEmpty(fp) {
			continue
		}
		return false
	}
	return true
}

func (wft *walkFileTree) relName(fpath string) string {
	// func Rel(basepath, targpath string) (string, error)
	// Rel函数返回一个相对路径，将basepath和该路径用路径分隔符连起来的新路径在词法上等价于targpath。
	// 也就是说，Join(basepath, Rel(basepath, targpath))等价于targpath本身。
	// 如果成功执行，返回值总是相对于basepath的，即使basepath和targpath没有共享的路径元素。
	// 如果两个参数一个是相对路径而另一个是绝对路径，或者targpath无法表示为相对于basepath的路径，将返回错误。
	name, _ := path.Rel(wft.prefix, fpath)
	return name
}

func (wft *walkFileTree) virPath(fpath string) string {
	name := fpath[len(wft.prefix):]
	if name == "" {
		return ""
	}
	name = name[1:]
	// func ToSlash(path string) string
	// ToSlash函数将path中的路径分隔符替换为斜杠（'/'）并返回替换结果，多个路径分隔符会替换为多个斜杠。
	name = path.ToSlash(name)
	return name
}

func (wft *walkFileTree) readDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	// func Sort(data Interface)
	// Sort排序data。
	// 它调用1次data.Len确定长度，调用O(n*log(n))次data.Less和data.Swap。
	// 本函数不能保证排序的稳定性（即不保证相等元素的相对次序不变）。
	sort.Sort(byName(list))
	return list, nil
}

func (wft *walkFileTree) walkLeaf(fpath string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if fpath == outputP {
		return nil
	}

	if fi.IsDir() {
		return nil
	}

	if ssym && fi.Mode()&os.ModeSymlink > 0 {
		return nil
	}

	name := wft.virPath(fpath)

	if wft.allfiles[name] {
		return nil
	}

	if added, err := wft.wak.compress(name, fpath, fi); added {
		if verbose {
			// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error)
			// Fprintf根据format参数生成格式化的字符串并写入w。
			// 返回写入的字节数和遇到的任何错误。
			fmt.Fprintf(w, "\t%s%scompressed%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", name, "\x1b[0m")
		}
		wft.allfiles[name] = true
		return err
	}
	return err
}

func (wft *walkFileTree) iterDirectory(fpath string, fi os.FileInfo) error {
	doFSym := fsym && fi.Mode()&os.ModeSymlink > 0
	if doFSym {
		nfi, err := os.Stat(fpath)
		if os.IsNotExist(err) {
			return nil
		}
		fi = nfi
	}

	relPath := wft.virPath(fpath)

	if len(relPath) > 0 {
		if wft.isExcludeName(fi.Name()) {
			return nil
		}

		if wft.isExclude(relPath) {
			return nil
		}
	}

	err := wft.walkLeaf(fpath, fi, nil)
	if err != nil {
		if fi.IsDir() && err == path.SkipDir {
			return nil
		}
		return err
	}

	if !fi.IsDir() {
		return nil
	}

	list, err := wft.readDir(fpath)
	if err != nil {
		return wft.walkLeaf(fpath, fi, err)
	}

	for _, fileInfo := range list {
		err = wft.iterDirectory(path.Join(fpath, fileInfo.Name()), fileInfo)
		if err != nil {
			if !fileInfo.IsDir() || err != path.SkipDir {
				return err
			}
		}
	}
	return nil
}

func (wft *walkFileTree) walkRoot(root string) error {
	wft.prefix = root
	fi, err := os.Stat(root)
	if err != nil {
		return err
	}
	return wft.iterDirectory(root, fi)
}

type tarWalk struct {
	walkFileTree
	tw *tar.Writer
}

func (wft *tarWalk) compress(name, fpath string, fi os.FileInfo) (bool, error) {
	isSym := fi.Mode()&os.ModeSymlink > 0
	link := ""
	if isSym {
		link, _ = os.Readlink(fpath)
	}

	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return false, err
	}
	hdr.Name = name

	tw := wft.tw
	err = tw.WriteHeader(hdr)
	if err != nil {
		return false, err
	}

	if isSym == false {
		fr, err := os.Open(fpath)
		if err != nil {
			return false, err
		}
		defer CloseFile(fr)
		_, err = io.Copy(tw, fr)
		if err != nil {
			return false, err
		}
		tw.Flush()
	}

	return true, nil
}

type zipWalk struct {
	walkFileTree
	zw *zip.Writer
}

func (wft *zipWalk) compress(name, fpath string, fi os.FileInfo) (bool, error) {
	isSym := fi.Mode()&os.ModeSymlink > 0

	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return false, err
	}
	hdr.Name = name

	zw := wft.zw
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return false, err
	}

	if isSym == false {
		fr, err := os.Open(fpath)
		if err != nil {
			return false, err
		}
		defer CloseFile(fr)
		_, err = io.Copy(w, fr)
		if err != nil {
			return false, err
		}
	} else {
		var link string
		if link, err = os.Readlink(fpath); err != nil {
			return false, err
		}
		_, err = w.Write([]byte(link))
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func packDirectory(excludePrefix []string, excludeSuffix []string,
	excludeRegexp []*regexp.Regexp, includePath ...string) (err error) {

	ColorLog("Excluding relpath prefix: %s\n", strings.Join(excludePrefix, ":"))
	ColorLog("Excluding relpath suffix: %s\n", strings.Join(excludeSuffix, ":"))
	if len(excludeRegexp) > 0 {
		ColorLog("Excluding filename regex: `%s`\n", strings.Join(excludeR, "`, `"))
	}

	w, err := os.OpenFile(outputP, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	var wft walker

	if format == "zip" {
		walk := new(zipWalk)
		zw := zip.NewWriter(w)
		defer func() {
			zw.Close()
		}()
		walk.allfiles = make(map[string]bool)
		walk.zw = zw
		walk.wak = walk
		walk.excludePrefix = excludePrefix
		walk.excludeSuffix = excludeSuffix
		walk.excludeRegexp = excludeRegexp
		wft = walk
	} else {
		walk := new(tarWalk)
		cw := gzip.NewWriter(w)
		tw := tar.NewWriter(cw)

		defer func() {
			tw.Flush()
			cw.Flush()
			tw.Close()
			cw.Close()
		}()
		walk.allfiles = make(map[string]bool)
		walk.tw = tw
		walk.wak = walk
		walk.excludePrefix = excludePrefix
		walk.excludeSuffix = excludeSuffix
		walk.excludeRegexp = excludeRegexp
		wft = walk
	}

	for _, p := range includePath {
		err = wft.walkRoot(p)
		if err != nil {
			return
		}
	}

	return
}

func isBeegoProject(thePath string) bool {
	fh, _ := os.Open(thePath)
	fis, _ := fh.Readdir(-1)
	regex := regexp.MustCompile(`(?s)package main.*?import.*?\(.*?github.com/astaxie/beego".*?\).*func main()`)
	for _, fi := range fis {
		if fi.IsDir() == false && strings.HasSuffix(fi.Name(), ".go") {
			data, err := ioutil.ReadFile(path.Join(thePath, fi.Name()))
			if err != nil {
				continue
			}
			if len(regex.Find(data)) > 0 {
				return true
			}
		}
	}
	return false
}

func packApp(cmd *Command, args []string) int {
	ShowShortVersionBanner()

	curPath, _ := os.Getwd()
	thePath := ""

	nArgs := []string{}
	has := false
	for _, a := range args {
		if a != "" && a[0] == '-' {
			has = true
		}
		if has {
			nArgs = append(nArgs, a)
		}
	}
	cmdPack.Flag.Parse(nArgs)

	if path.IsAbs(appPath) == false {
		appPath = path.Join(curPath, appPath)
	}

	thePath, err := path.Abs(appPath)
	if err != nil {
		exitPrint(fmt.Sprintf("Wrong app path: %s", thePath))
	}
	if stat, err := os.Stat(thePath); os.IsNotExist(err) || stat.IsDir() == false {
		exitPrint(fmt.Sprintf("App path does not exist: %s", thePath))
	}

	if isBeegoProject(thePath) == false {
		exitPrint(fmt.Sprintf("Bee does not support non Beego project"))
	}

	ColorLog("Packaging application: %s\n", thePath)

	appName := path.Base(thePath)

	goos := runtime.GOOS
	if v, found := syscall.Getenv("GOOS"); found {
		goos = v
	}
	goarch := runtime.GOARCH
	if v, found := syscall.Getenv("GOARCH"); found {
		goarch = v
	}

	str := strconv.FormatInt(time.Now().UnixNano(), 10)[9:]

	tmpdir := path.Join(os.TempDir(), "beePack-"+str)

	os.Mkdir(tmpdir, 0700)

	if build {
		ColorLog("Building application...\n")
		var envs []string
		for _, env := range buildEnvs {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				if len(k) > 0 && len(v) > 0 {
					switch k {
					case "GOOS":
						goos = v
					case "GOARCH":
						goarch = v
					default:
						envs = append(envs, fmt.Sprintf("%s=%s", k, v))
					}
				}
			}
		}

		os.Setenv("GOOS", goos)
		os.Setenv("GOARCH", goarch)

		ColorLog("Env: GOOS=%s GOARCH=%s\n", goos, goarch)

		binPath := path.Join(tmpdir, appName)
		if goos == "windows" {
			binPath += ".exe"
		}

		args := []string{"build", "-o", binPath}
		if len(buildArgs) > 0 {
			args = append(args, strings.Fields(buildArgs)...)
		}

		if verbose {
			fmt.Fprintf(w, "\t%s%s+ go %s%s%s\n", "\x1b[32m", "\x1b[1m", strings.Join(args, " "), "\x1b[21m", "\x1b[0m")
		}

		execmd := exec.Command("go", args...)
		execmd.Env = append(os.Environ(), envs...)
		execmd.Stdout = os.Stdout
		execmd.Stderr = os.Stderr
		execmd.Dir = thePath
		err = execmd.Run()
		if err != nil {
			exitPrint(err.Error())
		}

		ColorLog("Build successful\n")
	}

	switch format {
	case "zip":
	default:
		format = "tar.gz"
	}

	outputN := appName + "." + format

	if outputP == "" || path.IsAbs(outputP) == false {
		outputP = path.Join(curPath, outputP)
	}

	if _, err := os.Stat(outputP); err != nil {
		err = os.MkdirAll(outputP, 0755)
		if err != nil {
			exitPrint(err.Error())
		}
	}

	outputP = path.Join(outputP, outputN)

	var exp, exs []string
	for _, p := range strings.Split(excludeP, ":") {
		if len(p) > 0 {
			exp = append(exp, p)
		}
	}
	for _, p := range strings.Split(excludeS, ":") {
		if len(p) > 0 {
			exs = append(exs, p)
		}
	}

	var exr []*regexp.Regexp
	for _, r := range excludeR {
		if len(r) > 0 {
			if re, err := regexp.Compile(r); err != nil {
				exitPrint(err.Error())
			} else {
				exr = append(exr, re)
			}
		}
	}

	err = packDirectory(exp, exs, exr, tmpdir, thePath)
	if err != nil {
		exitPrint(err.Error())
	}

	ColorLog("Writing to output: `%s`\n", outputP)
	return 0
}
