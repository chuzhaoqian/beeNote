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
	"fmt" // 格式化i/o
	"os"	// 系统函数
	"path" // 斜杠路径操作函数
	"strings" // 字符串简单操作函数
	"time"	// 时间函数
)

const (
	MPath       = "migrations"
	MDateFormat = "20060102_150405"
	DBPath      = "database"
)

type DBDriver interface {
	generateCreateUp(tableName string) string
	generateCreateDown(tableName string) string
}

type mysqlDriver struct{}

func (m mysqlDriver) generateCreateUp(tableName string) string {
	upsql := `m.SQL("CREATE TABLE ` + tableName + "(" + m.generateSQLFromFields(fields.String()) + `)");`
	return upsql
}

func (m mysqlDriver) generateCreateDown(tableName string) string {
	downsql := `m.SQL("DROP TABLE ` + "`" + tableName + "`" + `")`
	return downsql
}

func (m mysqlDriver) generateSQLFromFields(fields string) string {
	sql, tags := "", ""
	// func Split(s, sep string) []string
	// 用去掉s中出现的sep的方式进行分割，会分割到结尾，并返回生成的所有片段组成的切片（每一个sep都会进行一次切割，即使两个sep相邻，也会进行两次切割）。
	// 如果sep为空字符，Split会将s切分成每一个unicode码值一个字符串。
	fds := strings.Split(fields, ",")
	for i, v := range fds {
		// func SplitN(s, sep string, n int) []string
		// 用去掉s中出现的sep的方式进行分割，会分割到结尾，并返回生成的所有片段组成的切片
		//（每一个sep都会进行一次切割，即使两个sep相邻，也会进行两次切割）。
		// 如果sep为空字符，Split会将s切分成每一个unicode码值一个字符串。参数n决定返回的切片的数目：
		// n > 0 : 返回的切片最多n个子字符串；最后一个子字符串包含未进行切割的部分。
		// n == 0: 返回nil
		// n < 0 : 返回所有的子字符串组成的切片
		kv := strings.SplitN(v, ":", 2)
		if len(kv) != 2 {
			ColorLog("[ERRO] Fields format is wrong. Should be: key:type,key:type " + v + "\n")
			return ""
		}
		typ, tag := m.getSQLType(kv[1])
		if typ == "" {
			ColorLog("[ERRO] Fields format is wrong. Should be: key:type,key:type " + v + "\n")
			return ""
		}
		// func ToLower(s string) string
		// 返回将所有字母都转为对应的小写版本的拷贝。
		if i == 0 && strings.ToLower(kv[0]) != "id" {
			sql += "`id` int(11) NOT NULL AUTO_INCREMENT,"
			tags = tags + "PRIMARY KEY (`id`),"
		}
		sql += "`" + snakeString(kv[0]) + "` " + typ + ","
		if tag != "" {
			// func Sprintf(format string, a ...interface{}) string
			// Sprintf根据format参数生成格式化的字符串并返回该字符串。
			tags = tags + fmt.Sprintf(tag, "`"+snakeString(kv[0])+"`") + ","
		}
	}
	// func TrimRight(s string, cutset string) string
	// 返回将s后端所有cutset包含的utf-8码值都去掉的字符串。
	sql = strings.TrimRight(sql+tags, ",")
	return sql
}

func (m mysqlDriver) getSQLType(ktype string) (tp, tag string) {
	kv := strings.SplitN(ktype, ":", 2)
	switch kv[0] {
	case "string":
		if len(kv) == 2 {
			return "varchar(" + kv[1] + ") NOT NULL", ""
		}
		return "varchar(128) NOT NULL", ""
	case "text":
		return "longtext  NOT NULL", ""
	case "auto":
		return "int(11) NOT NULL AUTO_INCREMENT", ""
	case "pk":
		return "int(11) NOT NULL", "PRIMARY KEY (%s)"
	case "datetime":
		return "datetime NOT NULL", ""
	case "int", "int8", "int16", "int32", "int64":
		fallthrough
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "int(11) DEFAULT NULL", ""
	case "bool":
		return "tinyint(1) NOT NULL", ""
	case "float32", "float64":
		return "float NOT NULL", ""
	case "float":
		return "float NOT NULL", ""
	}
	return "", ""
}

type postgresqlDriver struct{}

func (m postgresqlDriver) generateCreateUp(tableName string) string {
	upsql := `m.SQL("CREATE TABLE ` + tableName + "(" + m.generateSQLFromFields(fields.String()) + `)");`
	return upsql
}

func (m postgresqlDriver) generateCreateDown(tableName string) string {
	downsql := `m.SQL("DROP TABLE ` + tableName + `")`
	return downsql
}

func (m postgresqlDriver) generateSQLFromFields(fields string) string {
	sql, tags := "", ""
	fds := strings.Split(fields, ",")
	for i, v := range fds {
		kv := strings.SplitN(v, ":", 2)
		if len(kv) != 2 {
			ColorLog("[ERRO] Fields format is wrong. Should be: key:type,key:type " + v + "\n")
			return ""
		}
		typ, tag := m.getSQLType(kv[1])
		if typ == "" {
			ColorLog("[ERRO] Fields format is wrong. Should be: key:type,key:type " + v + "\n")
			return ""
		}
		if i == 0 && strings.ToLower(kv[0]) != "id" {
			sql += "id serial primary key,"
		}
		sql += snakeString(kv[0]) + " " + typ + ","
		if tag != "" {
			tags = tags + fmt.Sprintf(tag, snakeString(kv[0])) + ","
		}
	}
	if tags != "" {
		sql = strings.TrimRight(sql+" "+tags, ",")
	} else {
		sql = strings.TrimRight(sql, ",")
	}
	return sql
}

func (m postgresqlDriver) getSQLType(ktype string) (tp, tag string) {
	kv := strings.SplitN(ktype, ":", 2)
	switch kv[0] {
	case "string":
		if len(kv) == 2 {
			return "char(" + kv[1] + ") NOT NULL", ""
		}
		return "TEXT NOT NULL", ""
	case "text":
		return "TEXT NOT NULL", ""
	case "auto", "pk":
		return "serial primary key", ""
	case "datetime":
		return "TIMESTAMP WITHOUT TIME ZONE NOT NULL", ""
	case "int", "int8", "int16", "int32", "int64":
		fallthrough
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer DEFAULT NULL", ""
	case "bool":
		return "boolean NOT NULL", ""
	case "float32", "float64", "float":
		return "numeric NOT NULL", ""
	}
	return "", ""
}

func newDBDriver() DBDriver {
	switch driver {
	case "mysql":
		return mysqlDriver{}
	case "postgres":
		return postgresqlDriver{}
	default:
		panic("driver not supported")
	}
}

// generateMigration generates migration file template for database schema update.
// The generated file template consists of an up() method for updating schema and
// a down() method for reverting the update.
func generateMigration(mname, upsql, downsql, curpath string) {
	// var (
	//     Stdin  = NewFile(uintptr(syscall.Stdin), "/dev/stdin")
	//     Stdout = NewFile(uintptr(syscall.Stdout), "/dev/stdout")
	//     Stderr = NewFile(uintptr(syscall.Stderr), "/dev/stderr")
	// )
	// Stdin、Stdout和Stderr是指向标准输入、标准输出、标准错误输出的文件描述符。
	w := NewColorWriter(os.Stdout)
	// func Join(elem ...string) string
	// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加斜杠。结果是经过简化的，所有的空字符串元素会被忽略。
	migrationFilePath := path.Join(curpath, DBPath, MPath)
	// func Stat(name string) (fi FileInfo, err error)
	// Stat返回一个描述name指定的文件对象的FileInfo。如果指定的文件对象是一个符号链接，返回的FileInfo描述该符号链接指向的文件的信息，本函数会尝试跳转该链接。如果出错，返回的错误值为*PathError类型。
	
	// func IsNotExist(err error) bool
	// 返回一个布尔值说明该错误是否表示一个文件或目录不存在。ErrNotExist和一些系统调用错误会使它返回真。
	if _, err := os.Stat(migrationFilePath); os.IsNotExist(err) {
		// create migrations directory
		// func MkdirAll(path string, perm FileMode) error
		// MkdirAll使用指定的权限和名称创建一个目录，包括任何必要的上级目录，并返回nil，否则返回错误。
		// 权限位perm会应用在每一个被本函数创建的目录上。如果path指定了一个已经存在的目录，MkdirAll不做任何操作并返回nil。
		if err := os.MkdirAll(migrationFilePath, 0777); err != nil {
			ColorLog("[ERRO] Could not create migration directory: %s\n", err)
			os.Exit(2)
		}
	}
	// create file
	// func Now() Time
	// Now返回当前本地时间。
	
	// func (t Time) Format(layout string) string
	// Format根据layout指定的格式返回t代表的时间点的格式化文本表示。layout定义了参考时间：
	today := time.Now().Format(MDateFormat)
	// func Join(elem ...string) string
	// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加斜杠。结果是经过简化的，所有的空字符串元素会被忽略。
	fpath := path.Join(migrationFilePath, fmt.Sprintf("%s_%s.go", today, mname))
	if f, err := os.OpenFile(fpath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666); err == nil {
		defer CloseFile(f)
		// func Replace(s, old, new string, n int) string
		// 返回将s中前n个不重叠old子串都替换为new的新字符串，如果n<0会替换所有old子串。
		content := strings.Replace(MigrationTPL, "{{StructName}}", camelCase(mname)+"_"+today, -1)
		content = strings.Replace(content, "{{CurrTime}}", today, -1)
		content = strings.Replace(content, "{{UpSQL}}", upsql, -1)
		content = strings.Replace(content, "{{DownSQL}}", downsql, -1)
		f.WriteString(content)
		// Run 'gofmt' on the generated source code
		formatSourceCode(fpath)
		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", fpath, "\x1b[0m")
	} else {
		ColorLog("[ERRO] Could not create migration file: %s\n", err)
		os.Exit(2)
	}
}

const MigrationTPL = `package main

import (
	"github.com/astaxie/beego/migration"
)

// DO NOT MODIFY
type {{StructName}} struct {
	migration.Migration
}

// DO NOT MODIFY
func init() {
	m := &{{StructName}}{}
	m.Created = "{{CurrTime}}"
	migration.Register("{{StructName}}", m)
}

// Run the migrations
func (m *{{StructName}}) Up() {
	// use m.SQL("CREATE TABLE ...") to make schema update
	{{UpSQL}}
}

// Reverse the migrations
func (m *{{StructName}}) Down() {
	// use m.SQL("DROP TABLE ...") to reverse schema update
	{{DownSQL}}
}
`
