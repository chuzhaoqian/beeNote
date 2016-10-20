/**********************************************************\
|                                                          |
|                          hprose                          |
|                                                          |
| Official WebSite: http://www.hprose.com/                 |
|                   http://www.hprose.org/                 |
|                                                          |
\**********************************************************/
/**********************************************************\
 *                                                        *
 * Build rpc application use Hprose base on beego         *
 *                                                        *
 * LastModified: Oct 13, 2014                             *
 * Author: Liu jian <laoliu@lanmv.com>                    *
 *                                                        *
\**********************************************************/

package main

import (
	"fmt"	// 格式化i/o
	"os"	// 系统函数
	path "path/filepath"	// 文件路径函数
	"strings"	// 字符串简单函数
)

var cmdHproseapp = &Command{
	// CustomFlags: true,
	UsageLine: "hprose [appname]",
	Short:     "create an rpc application use hprose base on beego framework",
	Long: `
create an rpc application use hprose base on beego framework

bee hprose [appname] [-tables=""] [-driver=mysql] [-conn=root:@tcp(127.0.0.1:3306)/test]
    -tables: a list of table names separated by ',', default is empty, indicating all tables
    -driver: [mysql | postgres | sqlite], the default is mysql
    -conn:   the connection string used by the driver, the default is ''
             e.g. for mysql:    root:@tcp(127.0.0.1:3306)/test
             e.g. for postgres: postgres://postgres:postgres@127.0.0.1:5432/postgres

if conn is empty will create a example rpc application. otherwise generate rpc application use hprose based on an existing database.

In the current path, will create a folder named [appname]

In the appname folder has the follow struct:

	├── conf
	│   └── app.conf
	├── main.go
	└── models
	    └── object.go
	    └── user.go
`,
}

var hproseconf = `appname = {{.Appname}}
httpport = 8080
runmode = dev
autorender = false
copyrequestbody = true
EnableDocs = true
`
var hproseMaingo = `package main

import (
	"{{.Appname}}/models"
	"github.com/hprose/hprose-go/hprose"

	"github.com/astaxie/beego"
)

func main() {
	service := hprose.NewHttpService()
	service.AddFunction("AddOne", models.AddOne)
	service.AddFunction("GetOne", models.GetOne)
	beego.Handler("/", service)
	beego.Run()
}
`

var hproseMainconngo = `package main

import (
	"{{.Appname}}/models"
	"github.com/hprose/hprose-go/hprose"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	{{.DriverPkg}}
)

func init() {
	orm.RegisterDataBase("default", "{{.DriverName}}", "{{.conn}}")
}

func main() {
	service := hprose.NewHttpService()
	{{HproseFunctionList}}
	beego.Handler("/", service)
	beego.Run()
}

`

var hproseModels = `package models

import (
	"errors"
	"strconv"
	"time"
)

var (
	Objects map[string]*Object
)

type Object struct {
	ObjectId   string
	Score      int64
	PlayerName string
}

func init() {
	Objects = make(map[string]*Object)
	Objects["hjkhsbnmn123"] = &Object{"hjkhsbnmn123", 100, "astaxie"}
	Objects["mjjkxsxsaa23"] = &Object{"mjjkxsxsaa23", 101, "someone"}
}

func AddOne(object Object) (ObjectId string) {
	object.ObjectId = "astaxie" + strconv.FormatInt(time.Now().UnixNano(), 10)
	Objects[object.ObjectId] = &object
	return object.ObjectId
}

func GetOne(ObjectId string) (object *Object, err error) {
	if v, ok := Objects[ObjectId]; ok {
		return v, nil
	}
	return nil, errors.New("ObjectId Not Exist")
}

func GetAll() map[string]*Object {
	return Objects
}

func Update(ObjectId string, Score int64) (err error) {
	if v, ok := Objects[ObjectId]; ok {
		v.Score = Score
		return nil
	}
	return errors.New("ObjectId Not Exist")
}

func Delete(ObjectId string) {
	delete(Objects, ObjectId)
}

`

var hproseModels2 = `package models

import (
	"errors"
	"strconv"
	"time"
)

var (
	UserList map[string]*User
)

func init() {
	UserList = make(map[string]*User)
	u := User{"user_11111", "astaxie", "11111", Profile{"male", 20, "Singapore", "astaxie@gmail.com"}}
	UserList["user_11111"] = &u
}

type User struct {
	Id       string
	Username string
	Password string
	Profile  Profile
}

type Profile struct {
	Gender  string
	Age     int
	Address string
	Email   string
}

func AddUser(u User) string {
	u.Id = "user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	UserList[u.Id] = &u
	return u.Id
}

func GetUser(uid string) (u *User, err error) {
	if u, ok := UserList[uid]; ok {
		return u, nil
	}
	return nil, errors.New("User not exists")
}

func GetAllUsers() map[string]*User {
	return UserList
}

func UpdateUser(uid string, uu *User) (a *User, err error) {
	if u, ok := UserList[uid]; ok {
		if uu.Username != "" {
			u.Username = uu.Username
		}
		if uu.Password != "" {
			u.Password = uu.Password
		}
		if uu.Profile.Age != 0 {
			u.Profile.Age = uu.Profile.Age
		}
		if uu.Profile.Address != "" {
			u.Profile.Address = uu.Profile.Address
		}
		if uu.Profile.Gender != "" {
			u.Profile.Gender = uu.Profile.Gender
		}
		if uu.Profile.Email != "" {
			u.Profile.Email = uu.Profile.Email
		}
		return u, nil
	}
	return nil, errors.New("User Not Exist")
}

func Login(username, password string) bool {
	for _, u := range UserList {
		if u.Username == username && u.Password == password {
			return true
		}
	}
	return false
}

func DeleteUser(uid string) {
	delete(UserList, uid)
}
`

var hproseAddFunctions = []string{}

func init() {
	cmdHproseapp.Run = createhprose
	// func (f *FlagSet) Var(value Value, name string, usage string)
	// Var方法使用指定的名字、使用信息注册一个flag。
	// 该flag的类型和值由第一个参数表示，该参数应实现了Value接口。
	// 例如，用户可以创建一个flag，可以用Value接口的Set方法将逗号分隔的字符串转化为字符串切片。
	cmdHproseapp.Flag.Var(&tables, "tables", "specify tables to generate model")
	cmdHproseapp.Flag.Var(&driver, "driver", "database driver: mysql, postgresql, etc.")
	cmdHproseapp.Flag.Var(&conn, "conn", "connection string used by the driver to connect to a database instance")
}

func createhprose(cmd *Command, args []string) int {
	ShowShortVersionBanner()

	w := NewColorWriter(os.Stdout)
	// func Getwd() (dir string, err error)
	// Getwd返回一个对应当前工作目录的根路径。
	// 如果当前目录可以经过多条路径抵达（因为硬链接），Getwd会返回其中一个。
	curpath, _ := os.Getwd()
	if len(args) > 1 {
		// func (f *FlagSet) Parse(arguments []string) error
		// 从arguments中解析注册的flag。
		// 必须在所有flag都注册好而未访问其值时执行。
		// 未注册却使用flag -help时，会返回ErrHelp。
		cmd.Flag.Parse(args[1:])
	}
	apppath, packpath, err := checkEnv(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if driver == "" {
		driver = "mysql"
	}
	if conn == "" {
	}

	ColorLog("[INFO] Creating Hprose application...\n")
	// func MkdirAll(path string, perm FileMode) error
	// MkdirAll使用指定的权限和名称创建一个目录，包括任何必要的上级目录，并返回nil，否则返回错误。
	// 权限位perm会应用在每一个被本函数创建的目录上。
	// 如果path指定了一个已经存在的目录，MkdirAll不做任何操作并返回nil。
	os.MkdirAll(apppath, 0755)
	// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error)
	// Fprintf根据format参数生成格式化的字符串并写入w。
	// 返回写入的字节数和遇到的任何错误。
	fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", apppath, "\x1b[0m")
	// func Mkdir(name string, perm FileMode) error
	// Mkdir使用指定的权限和名称创建一个目录。
	// 如果出错，会返回*PathError底层类型的错误。
	os.Mkdir(path.Join(apppath, "conf"), 0755)
	fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "conf"), "\x1b[0m")
	fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "conf", "app.conf"), "\x1b[0m")
	// func Join(elem ...string) string
	// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加路径分隔符。
	// 结果是经过简化的，所有的空字符串元素会被忽略。
	
	// func Replace(s, old, new string, n int) string
	// 返回将s中前n个不重叠old子串都替换为new的新字符串，如果n<0会替换所有old子串。
	WriteToFile(path.Join(apppath, "conf", "app.conf"),
		strings.Replace(hproseconf, "{{.Appname}}", args[0], -1))

	if conn != "" {
		ColorLog("[INFO] Using '%s' as 'driver'\n", driver)
		ColorLog("[INFO] Using '%s' as 'conn'\n", conn)
		ColorLog("[INFO] Using '%s' as 'tables'\n", tables)
		generateHproseAppcode(string(driver), string(conn), "1", string(tables), path.Join(curpath, args[0]))
		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "main.go"), "\x1b[0m")
		maingoContent := strings.Replace(hproseMainconngo, "{{.Appname}}", packpath, -1)
		maingoContent = strings.Replace(maingoContent, "{{.DriverName}}", string(driver), -1)
		maingoContent = strings.Replace(maingoContent, "{{HproseFunctionList}}", strings.Join(hproseAddFunctions, ""), -1)
		if driver == "mysql" {
			maingoContent = strings.Replace(maingoContent, "{{.DriverPkg}}", `_ "github.com/go-sql-driver/mysql"`, -1)
		} else if driver == "postgres" {
			maingoContent = strings.Replace(maingoContent, "{{.DriverPkg}}", `_ "github.com/lib/pq"`, -1)
		}
		WriteToFile(path.Join(apppath, "main.go"),
			strings.Replace(
				maingoContent,
				"{{.conn}}",
				conn.String(),
				-1,
			),
		)
	} else {
		os.Mkdir(path.Join(apppath, "models"), 0755)
		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "models"), "\x1b[0m")

		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "models", "object.go"), "\x1b[0m")
		WriteToFile(path.Join(apppath, "models", "object.go"), apiModels)

		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "models", "user.go"), "\x1b[0m")
		WriteToFile(path.Join(apppath, "models", "user.go"), apiModels2)

		fmt.Fprintf(w, "\t%s%screate%s\t %s%s\n", "\x1b[32m", "\x1b[1m", "\x1b[21m", path.Join(apppath, "main.go"), "\x1b[0m")
		WriteToFile(path.Join(apppath, "main.go"),
			strings.Replace(hproseMaingo, "{{.Appname}}", packpath, -1))
	}
	ColorLog("[SUCC] New Hprose application successfully created!\n")
	return 0
}
