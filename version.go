package main

import (
	"bufio" // 实现了有缓冲的I/O
	"bytes" // 实现了操作[]byte的常用函数。本包的函数和strings包的函数相当类似。
	"fmt"	// 格式化I/O包
	"io" // I/O原语的基本接口
	"os" // 提供操作系统函数
	path "path/filepath" // 实现了兼容各操作系统的文件路径的实用操作函数。别名定义为path
	"regexp" // 正则表达式采用RE2语法
)

var cmdVersion = &Command{
	UsageLine: "version",
	Short:     "prints the current Bee version",
	Long: `
Prints the current Bee, Beego and Go version alongside the platform information

`,
}

const verboseVersionBanner string = `%s%s______
| ___ \
| |_/ /  ___   ___
| ___ \ / _ \ / _ \
| |_/ /|  __/|  __/
\____/  \___| \___| v{{ .BeeVersion }}%s
%s%s
├── Beego     : {{ .BeegoVersion }}
├── GoVersion : {{ .GoVersion }}
├── GOOS      : {{ .GOOS }}
├── GOARCH    : {{ .GOARCH }}
├── NumCPU    : {{ .NumCPU }}
├── GOPATH    : {{ .GOPATH }}
├── GOROOT    : {{ .GOROOT }}
├── Compiler  : {{ .Compiler }}
└── Date      : {{ Now "Monday, 2 Jan 2006" }}%s
`

const shortVersionBanner = `%s%s______
| ___ \
| |_/ /  ___   ___
| ___ \ / _ \ / _ \
| |_/ /|  __/|  __/
\____/  \___| \___| v{{ .BeeVersion }}%s
`

func init() {
	cmdVersion.Run = versionCmd
}

func versionCmd(cmd *Command, args []string) int {
	ShowVerboseVersionBanner() //调用本文件函数 
	return 0
}

// ShowVerboseVersionBanner prints the verbose version banner
func ShowVerboseVersionBanner() {
	// ./color.go
	w := NewColorWriter(os.Stdout)
	
	//func Sprintf(format string, a ...interface{}) string
	//Sprintf根据format参数生成格式化的字符串并返回该字符串。
	coloredBanner := fmt.Sprintf(verboseVersionBanner, "\x1b[35m", "\x1b[1m", "\x1b[0m",
		"\x1b[32m", "\x1b[1m", "\x1b[0m")
	//func NewBufferString(s string) *Buffer
	//NewBuffer使用s作为初始内容创建并初始化一个Buffer。
	//本函数用于创建一个用于读取已存在数据的buffer。
	//大多数情况下，new(Buffer)（或只是声明一个Buffer类型变量）就足以初始化一个Buffer了。
	InitBanner(w, bytes.NewBufferString(coloredBanner))
}

// ShowShortVersionBanner prints the short version banner
func ShowShortVersionBanner() {
	w := NewColorWriter(os.Stdout)
	coloredBanner := fmt.Sprintf(shortVersionBanner, "\x1b[35m", "\x1b[1m", "\x1b[0m")
	InitBanner(w, bytes.NewBufferString(coloredBanner))
}

func getBeegoVersion() string {
	//func Getenv(key string) string
	//Getenv检索并返回名为key的环境变量的值。如果不存在该环境变量会返回空字符串。
	gopath := os.Getenv("GOPATH")
	//func Compile(expr string) (*Regexp, error)
	//Compile解析并返回一个正则表达式。
	//如果成功返回，该Regexp就可用于匹配文本。
	//在匹配文本时，该正则表达式会尽可能早的开始匹配，并且在匹配过程中选择回溯搜索到的第一个匹配结果。
	//这种模式被称为“leftmost-first”，Perl、Python和其他实现都采用了这种模式，但本包的实现没有回溯的损耗。
	//对POSIX的“leftmost-longest”模式，参见CompilePOSIX。
	re, err := regexp.Compile(`VERSION = "([0-9.]+)"`)
	if err != nil {
		return ""
	}
	if gopath == "" {
		//func Errorf(format string, a ...interface{}) error
		//Errorf根据format参数生成格式化字符串并返回一个包含该字符串的错误。
		err = fmt.Errorf("You should set GOPATH env variable")
		return ""
	}
	//func SplitList(path string) []string
	//将PATH或GOPATH等环境变量里的多个路径分割开（这些路径被OS特定的表分隔符连接起来）。
	//与strings.Split函数的不同之处是：对""，SplitList返回[]string{}，而strings.Split返回[]string{""}。
	wgopath := path.SplitList(gopath)
	for _, wg := range wgopath {
		//func EvalSymlinks(path string) (string, error)
		//EvalSymlinks函数返回path指向的符号链接（软链接）所包含的路径。
		//如果path和返回值都是相对路径，会相对于当前目录；除非两个路径其中一个是绝对路径。
		
		//func Join(elem ...string) string
		//Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加路径分隔符。
		//结果是经过简化的，所有的空字符串元素会被忽略。
		wg, _ = path.EvalSymlinks(path.Join(wg, "src", "github.com", "astaxie", "beego"))
		filename := path.Join(wg, "beego.go")
		//func Stat(name string) (fi FileInfo, err error)
		//Stat返回一个描述name指定的文件对象的FileInfo。
		//如果指定的文件对象是一个符号链接，返回的FileInfo描述该符号链接指向的文件的信息，本函数会尝试跳转该链接。
		//如果出错，返回的错误值为*PathError类型。
		_, err := os.Stat(filename)
		if err != nil {
			//func IsNotExist(err error) bool
			//返回一个布尔值说明该错误是否表示一个文件或目录不存在。
			//ErrNotExist和一些系统调用错误会使它返回真。
			if os.IsNotExist(err) {
				continue
			}
			ColorLog("[ERRO] Get `beego.go` has error\n")
		}
		//func Open(name string) (file *File, err error)
		//Open打开一个文件用于读取。
		//如果操作成功，返回的文件对象的方法可用于读取数据；
		//对应的文件描述符具有O_RDONLY模式。
		//如果出错，错误底层类型是*PathError。
		fd, err := os.Open(filename)
		if err != nil {
			ColorLog("[ERRO] Open `beego.go` has error\n")
			continue
		}
		//func NewReader(rd io.Reader) *Reader
		//NewReader创建一个具有默认大小缓冲、从r读取的*Reader。
		reader := bufio.NewReader(fd)
		for {
			//func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error)
			//ReadLine是一个低水平的行数据读取原语。
			//大多数调用者应使用ReadBytes('\n')或ReadString('\n')代替，或者使用Scanner。
			byteLine, _, er := reader.ReadLine()
			if er != nil && er != io.EOF {
				return ""
			}
			//var EOF = errors.New("EOF")
			//EOF当无法得到更多输入时，Read方法返回EOF。
			//当函数一切正常的到达输入的结束时，就应返回EOF。
			//如果在一个结构化数据流中EOF在不期望的位置出现了，则应返回错误ErrUnexpectedEOF或者其它给出更多细节的错误。
			if er == io.EOF {
				break
			}
			line := string(byteLine)
			//func (re *Regexp) FindSubmatchIndex(b []byte) []int
			//Find返回一个保管正则表达式re在b中的最左侧的一个匹配结果以及（可能有的）分组匹配的结果的起止位置的切片。
			//匹配结果和分组匹配结果可以通过起止位置对b做切片操作得到：b[loc[2*n]:loc[2*n+1]]。
			//如果没有匹配到，会返回nil。
			s := re.FindStringSubmatch(line)
			if len(s) >= 2 {
				return s[1]
			}
		}

	}
	return "Beego not installed. Please install it first: https://github.com/astaxie/beego"
}
