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

// Bee is a tool for developling applications based on beego framework.
package main

import (
	"flag"	//命令行参数的解析
	"fmt"	//格式化I/O包
	"html/template"	//数据驱动模板，用于生成可对抗代码注入的安全HTML输出
	"io"	//I/O原语的基本接口
	"log"	//简单的日志服务
	"os"	//提供操作系统函数
	"strings"	//实现了用于操作字符的简单函数
)

const version = "1.5.2"	//当前版本

type Command struct {
	// Run runs the command. 运行命令
	// The args are the arguments after the command name. args是命令后面的参数
	Run func(cmd *Command, args []string) int

	// UsageLine is the one-line usage message. 
	// The first word in the line is taken to be the command name.
	UsageLine string

	// Short is the short description shown in the 'go help' output.
	// type HTML string
	// 用于封装一个已知安全的HTML文档片段。
	// 它不应被第三方使用，也不能用于含有未闭合的标签或注释的HTML文本。
	// 该类型适用于封装一个效果良好的HTML生成器生成的HTML文本或者本包模板的输出的文本。
	Short template.HTML

	// Long is the long message shown in the 'go help <this-command>' output.
	Long template.HTML

	// Flag is a set of flags specific to this command.
	// flag.FlagSet 代表一个已注册的flag的集合
	Flag flag.FlagSet

	// CustomFlags indicates that the command will do its own
	// flag parsing.
	CustomFlags bool
}

// Name returns the command's name: the first word in the usage line.
//返回命令的名字，c *Command.UsageLine的第一个单词或者本身
func (c *Command) Name() string {
	name := c.UsageLine
	// func Index(s, sep string) int	
	// 子串sep在字符串s中第一次出现的位置，不存在则返回-1。
	if i >= 0 {
	i := strings.Index(name, " ")
		name = name[:i]	//切片
	}
	return name
}

//输出命令 并退出
func (c *Command) Usage() {
	// os.Stderr 标准错误输出的文件描述符
	fmt.Fprintf(os.Stderr, "usage: %s\n\n", c.UsageLine)
	// func TrimSpace(s string) string
	// 返回将s前后端所有空白（unicode.IsSpace指定）都去掉的字符串。
	fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSpace(string(c.Long)))
	// func Exit(code int)
	// Exit让当前程序以给出的状态码code退出。一般来说，状态码0表示成功，非0表示出错。程序会立刻终止，defer的函数不会被执行。
	os.Exit(2)
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command such as importpath.
// 是否可运行
func (c *Command) Runnable() bool {
	return c.Run != nil
}

// 切片 在其他文件中定义
var commands = []*Command{
	cmdNew,	// ./new.go
	cmdRun,	// ./run.go
	cmdPack, // ./pack.go
	cmdApiapp, // ./apiapp.go
	cmdHproseapp, // ./hproseapp.go
	//cmdRouter,
	//cmdTest,
	cmdBale, // ./bale.go
	cmdVersion, // ./version.go
	cmdGenerate, // ./g.go
	//cmdRundocs,
	cmdMigrate, // ./migrate.go
	cmdFix, // ./fix.go
}

func main() {
	// flag.Usage 制定到一个自定义函数 usage()
	flag.Usage = usage
	// 从os.Args[1:]中解析注册的flag。必须在所有flag都注册好而未访问其值时执行。未注册却使用flag -help时，会返回ErrHelp。
	flag.Parse()
	// func (l *Logger) SetPrefix(prefix string)
	// 设置logger的输出前缀。
	log.SetFlags(0)
	// func (f *FlagSet) Args() []string
	// 返回解析之后剩下的非flag参数。（不包括命令名）
	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	if args[0] == "help" {
		help(args[1:])
		return
	}

	for _, cmd := range commands {
		if cmd.Name() == args[0] && cmd.Run != nil {
			cmd.Flag.Usage = func() { cmd.Usage() }
			if cmd.CustomFlags {
				args = args[1:]
			} else {
				cmd.Flag.Parse(args[1:])
				args = cmd.Flag.Args()
			}
			os.Exit(cmd.Run(cmd, args))
			return
		}
	}
	
	// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error)
	// 根据format参数生成格式化的字符串并写入w。返回写入的字节数和遇到的任何错误。
	fmt.Fprintf(os.Stderr, "bee: unknown subcommand %q\nRun 'bee help' for usage.\n", args[0])
	os.Exit(2)
}

var usageTemplate = `Bee is a tool for managing beego framework.

Usage:

	bee command [arguments]

The commands are:
{{range .}}{{if .Runnable}}
    {{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "bee help [command]" for more information about a command.

Additional help topics:
{{range .}}{{if not .Runnable}}
    {{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "bee help [topic]" for more information about that topic.

`

var helpTemplate = `{{if .Runnable}}usage: bee {{.UsageLine}}

{{end}}{{.Long | trim}}
`

func usage() {
	// os.Stdout 标准输出
	tmpl(os.Stdout, usageTemplate, commands)
	os.Exit(2)
}

// io.Writer 接口用于包装基本的写入方法。
func tmpl(w io.Writer, text string, data interface{}) {
	// func New(name string) *Template
	// 创建一个名为name的模板。
	t := template.New("top")
	
	// func (t *Template) Funcs(funcMap FuncMap) *Template
	//向模板t的函数字典里加入参数funcMap内的键值对。
	// 如果funcMap某个键值对的值不是函数类型或者返回值不符合要求会panic。
	// 但是，可以对t函数列表的成员进行重写。方法返回t以便进行链式调用。
	
	// type FuncMap map[string]interface{}
	// 定义了函数名字符串到函数的映射，每个函数都必须有1到2个返回值，如果有2个则后一个必须是error接口类型；
	// 如果有2个返回值的方法返回的error非nil，模板执行会中断并返回给调用者该错误。
	// 该类型拷贝自text/template包的同名类型，因此不需要导入该包以使用该类型。
	
	// type HTML string
	// 用于封装一个已知安全的HTML文档片段。
	// 它不应被第三方使用，也不能用于含有未闭合的标签或注释的HTML文本。
	// 该类型适用于封装一个效果良好的HTML生成器生成的HTML文本或者本包模板的输出的文本。
	t.Funcs(template.FuncMap{"trim": func(s template.HTML) template.HTML {
		return template.HTML(strings.TrimSpace(string(s)))
	}})
	
	// func Must(t *Template, err error) *Template
	// Must函数用于包装返回(*Template, error)的函数/方法调用，它会在err非nil时panic，一般用于变量初始化：
	// var t = template.Must(template.New("name").Parse("html"))
	
	// func (t *Template) Parse(src string) (*Template, error)
	// 将字符串text解析为模板。嵌套定义的模板会关联到最顶层的t。
	// Parse可以多次调用，但只有第一次调用可以包含空格、注释和模板定义之外的文本。
	// 如果后面的调用在解析后仍剩余文本会引发错误、返回nil且丢弃剩余文本；如果解析得到的模板已有相关联的同名模板，会覆盖掉原模板。
	template.Must(t.Parse(text))
	
	// func (t *Template) Execute(wr io.Writer, data interface{}) error
	// 将解析好的模板应用到data上，并将输出写入wr。
	// 如果执行时出现错误，会停止执行，但有可能已经写入wr部分数据。模板可以安全的并发执行。
	if err := t.Execute(w, data); err != nil {
		panic(err)
	}
}

func help(args []string) {
	if len(args) == 0 {
		usage()
		// not exit 2: succeeded at 'go help'.
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stdout, "usage: bee help command\n\nToo many arguments given.\n")
		os.Exit(2) // failed at 'bee help'
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name() == arg {
			tmpl(os.Stdout, helpTemplate, cmd)
			// not exit 2: succeeded at 'go help cmd'.
			return
		}
	}
	
	// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error)
	// 根据format参数生成格式化的字符串并写入w。返回写入的字节数和遇到的任何错误。
	fmt.Fprintf(os.Stdout, "Unknown help topic %#q.  Run 'bee help'.\n", arg)
	os.Exit(2) // failed at 'bee help cmd'
}
