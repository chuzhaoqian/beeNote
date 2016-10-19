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
		// var SkipDir = errors.New("skip this directory")
		// 用作WalkFunc类型的返回值，表示该次调用的path参数指定的目录应被跳过。
		// 本错误不应被任何其他函数返回。
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
		// func Readlink(name string) (string, error)
		// Readlink获取name指定的符号链接文件指向的文件的路径。
		// 如果出错，会返回*PathError底层类型的错误。
		link, _ = os.Readlink(fpath)
	}
	// func FileInfoHeader(fi os.FileInfo, link string) (*Header, error)
	// FileInfoHeader返回一个根据fi填写了部分字段的Header。 
	// 如果fi描述一个符号链接，FileInfoHeader函数将link参数作为链接目标。
	// 如果fi描述一个目录，会在名字后面添加斜杠。
	// 因为os.FileInfo接口的Name方法只返回它描述的文件的无路径名，有可能需要将返回值的Name字段修改为文件的完整路径名。
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return false, err
	}
	// type Header struct {
    	// Name       string    // 记录头域的文件名
	hdr.Name = name
	
	// wft.tw 为 *tar.Writer
	// Writer类型提供了POSIX.1格式的tar档案文件的顺序写入。
	// 一个tar档案文件包含一系列文件。
	// 调用WriteHeader来写入一个新的文件，然后调用Write写入文件的数据，该记录写入的数据不能超过hdr.Size字节。
	tw := wft.tw
	// func (tw *Writer) WriteHeader(hdr *Header) error
	// WriteHeader写入hdr并准备接受文件内容。
	// 如果不是第一次调用本方法，会调用Flush。
	// 在Close之后调用本方法会返回ErrWriteAfterClose。
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
		// func Copy(dst Writer, src Reader) (written int64, err error)
		// 将src的数据拷贝到dst，直到在src上到达EOF或发生错误。
		// 返回拷贝的字节数和遇到的第一个错误。
		// 对成功的调用，返回值err为nil而非EOF，因为Copy定义为从src读取直到EOF，它不会将读取到EOF视为应报告的错误。
		// 如果src实现了WriterTo接口，本函数会调用src.WriteTo(dst)进行拷贝；
		// 否则如果dst实现了ReaderFrom接口，本函数会调用dst.ReadFrom(src)进行拷贝。
		_, err = io.Copy(tw, fr)
		if err != nil {
			return false, err
		}
		// func (tw *Writer) Flush() error
		// Flush结束当前文件的写入。（可选的）
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
	// func FileInfoHeader(fi os.FileInfo) (*FileHeader, error)
	// FileInfoHeader返回一个根据fi填写了部分字段的Header。
	// 因为os.FileInfo接口的Name方法只返回它描述的文件的无路径名，有可能需要将返回值的Name字段修改为文件的完整路径名。
	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return false, err
	}
	
	// type FileHeader struct {
	//Name string
	// Name是文件名，它必须是相对路径，不能以设备或斜杠开始，只接受'/'作为路径分隔符
	hdr.Name = name
	
	//*zip.Writer
	// Writer类型实现了zip文件的写入器。
	zw := wft.zw
	// func (w *Writer) CreateHeader(fh *FileHeader) (io.Writer, error)
	// 使用给出的*FileHeader来作为文件的元数据添加一个文件进zip文件。
	// 本方法返回一个io.Writer接口（用于写入新添加文件的内容）。
	// 新增文件的内容必须在下一次调用CreateHeader、Create或Close方法之前全部写入。
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
		// type Writer interface {
    			//Write(p []byte) (n int, err error)
		//	}
		// Writer接口用于包装基本的写入方法。
		// Write方法len(p) 字节数据从p写入底层的数据流。
		// 它会返回写入的字节数(0 <= n <= len(p))和遇到的任何导致写入提取结束的错误。
		// Write必须返回非nil的错误，如果它返回的 n < len(p)。
		// Write不能修改切片p中的数据，即使临时修改也不行。
		_, err = w.Write([]byte(link))
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func packDirectory(excludePrefix []string, excludeSuffix []string,
	excludeRegexp []*regexp.Regexp, includePath ...string) (err error) {
	// ./util.go
	ColorLog("Excluding relpath prefix: %s\n", strings.Join(excludePrefix, ":"))
	ColorLog("Excluding relpath suffix: %s\n", strings.Join(excludeSuffix, ":"))
	if len(excludeRegexp) > 0 {
		ColorLog("Excluding filename regex: `%s`\n", strings.Join(excludeR, "`, `"))
	}
	// func OpenFile(name string, flag int, perm FileMode) (file *File, err error)
	// OpenFile是一个更一般性的文件打开函数，大多数调用者都应用Open或Create代替本函数。
	// 它会使用指定的选项（如O_RDONLY等）、指定的模式（如0666等）打开指定名称的文件。如果操作成功，返回的文件对象可用于I/O。
	// 如果出错，错误底层类型是*PathError。
	// O_RDONLY int = syscall.O_RDONLY // 只读模式打开文件
	// O_WRONLY int = syscall.O_WRONLY // 只写模式打开文件
	// O_RDWR   int = syscall.O_RDWR   // 读写模式打开文件
	// O_APPEND int = syscall.O_APPEND // 写操作时将数据附加到文件尾部
	// O_CREATE int = syscall.O_CREAT  // 如果不存在将创建一个新文件
	// O_EXCL   int = syscall.O_EXCL   // 和O_CREATE配合使用，文件必须不存在
	// O_SYNC   int = syscall.O_SYNC   // 打开文件用于同步I/O
	// O_TRUNC  int = syscall.O_TRUNC  // 如果可能，打开时清空文件
	w, err := os.OpenFile(outputP, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	var wft walker

	if format == "zip" {
		walk := new(zipWalk)
		// func NewWriter(w io.Writer) *Writer
		// NewWriter创建并返回一个将zip文件写入w的*Writer。
		zw := zip.NewWriter(w)
		defer func() {
			// func (w *Writer) Close() error
			// Close方法通过写入中央目录关闭该*Writer。本方法不会也没办法关闭下层的io.Writer接口。
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
		// func NewWriter(w io.Writer) *Writer
		// NewWriter创建并返回一个Writer。
		// 写入返回值的数据都会在压缩后写入w。
		// 调用者有责任在结束写入后调用返回值的Close方法。
		// 因为写入的数据可能保存在缓冲中没有刷新入下层。
		// 如要设定Writer.Header字段，调用者必须在第一次调用Write方法或者Close方法之前设置。
		// Header字段的Comment和Name字段是go的utf-8字符串，但下层格式要求为NUL中止的ISO 8859-1 (Latin-1)序列。
		// 如果这两个字段的字符串包含NUL或非Latin-1字符，将导致Write方法返回错误。
		cw := gzip.NewWriter(w)
		// func NewWriter(w io.Writer) *Writer
		// NewWriter创建一个写入w的*Writer。
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
	// func MustCompile(str string) *Regexp
	// 解析并返回一个正则表达式。如果成功返回，该Regexp就可用于匹配文本。
	// MustCompile类似Compile但会在解析失败时panic，主要用于全局正则表达式变量的安全初始化。
	regex := regexp.MustCompile(`(?s)package main.*?import.*?\(.*?github.com/astaxie/beego".*?\).*func main()`)
	for _, fi := range fis {
		// func (m FileMode) IsDir() bool
		// IsDir报告m是否是一个目录。
		if fi.IsDir() == false && strings.HasSuffix(fi.Name(), ".go") {
			// func ReadFile(filename string) ([]byte, error)
			// ReadFile 从filename指定的文件中读取数据并返回文件的内容。
			// 成功的调用返回的err为nil而非EOF。
			// 因为本函数定义为读取整个文件，它不会将读取返回的EOF视为应报告的错误。
			data, err := ioutil.ReadFile(path.Join(thePath, fi.Name()))
			if err != nil {
				continue
			}
			// func (re *Regexp) Find(b []byte) []byte
			// Find返回保管正则表达式re在b中的最左侧的一个匹配结果的[]byte切片。
			// 如果没有匹配到，会返回nil。
			if len(regex.Find(data)) > 0 {
				return true
			}
		}
	}
	return false
}

func packApp(cmd *Command, args []string) int {
	ShowShortVersionBanner()
	// func Getwd() (dir string, err error)
	// Getwd返回一个对应当前工作目录的根路径。
	// 如果当前目录可以经过多条路径抵达（因为硬链接），Getwd会返回其中一个。
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
	// func (f *FlagSet) Parse(arguments []string) error
	// 从arguments中解析注册的flag。
	// 必须在所有flag都注册好而未访问其值时执行。
	// 未注册却使用flag -help时，会返回ErrHelp。
	cmdPack.Flag.Parse(nArgs)
	// func IsAbs(path string) bool
	// IsAbs返回路径是否是一个绝对路径。
	if path.IsAbs(appPath) == false {
		appPath = path.Join(curPath, appPath)
	}
	// func Abs(path string) (string, error)
	// Abs函数返回path代表的绝对路径，如果path不是绝对路径，会加入当前工作目录以使之成为绝对路径。
	// 因为硬链接的存在，不能保证返回的绝对路径是唯一指向该地址的绝对路径。
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
