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
	"database/sql"	// sql包提供了保证SQL或类SQL数据库的泛用接口。
			// 使用sql包时必须注入（至少）一个数据库驱动
	"fmt"	// 格式化i/o
	"os"	// 系统函数
	"os/exec"	// 执行外部命令。
	"path"	// 对斜杠分隔的路径的实用操作函数
	"strconv"	// 基本数据类型和其字符串表示的相互转换
	"strings"	// 字符简单函数
	"time"	// 时间的显示和测量用的函数。日历的计算采用的是公历。
	"runtime"	// go运行时环境的互操作
)

var cmdMigrate = &Command{
	UsageLine: "migrate [Command]",
	Short:     "run database migrations",
	Long: `
bee migrate [-driver=mysql] [-conn="root:@tcp(127.0.0.1:3306)/test"]
    run all outstanding migrations
    -driver: [mysql | postgres | sqlite] (default: mysql)
    -conn:   the connection string used by the driver, the default is root:@tcp(127.0.0.1:3306)/test

bee migrate rollback [-driver=mysql] [-conn="root:@tcp(127.0.0.1:3306)/test"]
    rollback the last migration operation
    -driver: [mysql | postgres | sqlite] (default: mysql)
    -conn:   the connection string used by the driver, the default is root:@tcp(127.0.0.1:3306)/test

bee migrate reset [-driver=mysql] [-conn="root:@tcp(127.0.0.1:3306)/test"]
    rollback all migrations
    -driver: [mysql | postgres | sqlite] (default: mysql)
    -conn:   the connection string used by the driver, the default is root:@tcp(127.0.0.1:3306)/test

bee migrate refresh [-driver=mysql] [-conn="root:@tcp(127.0.0.1:3306)/test"]
    rollback all migrations and run them all again
    -driver: [mysql | postgres | sqlite] (default: mysql)
    -conn:   the connection string used by the driver, the default is root:@tcp(127.0.0.1:3306)/test
`,
}

var mDriver docValue
var mConn docValue

func init() {
	cmdMigrate.Run = runMigration
	// func (f *FlagSet) Var(value Value, name string, usage string)
	// Var方法使用指定的名字、使用信息注册一个flag。
	// 该flag的类型和值由第一个参数表示，该参数应实现了Value接口。
	// 例如，用户可以创建一个flag，可以用Value接口的Set方法将逗号分隔的字符串转化为字符串切片。
	cmdMigrate.Flag.Var(&mDriver, "driver", "database driver: mysql, postgres, sqlite, etc.")
	cmdMigrate.Flag.Var(&mConn, "conn", "connection string used by the driver to connect to a database instance")
}

// runMigration is the entry point for starting a migration
func runMigration(cmd *Command, args []string) int {
	ShowShortVersionBanner()
	// func Getwd() (dir string, err error)
	// Getwd返回一个对应当前工作目录的根路径。
	// 如果当前目录可以经过多条路径抵达（因为硬链接），Getwd会返回其中一个。
	currpath, _ := os.Getwd()

	gps := GetGOPATHs()
	if len(gps) == 0 {
		ColorLog("[ERRO] Fail to start[ %s ]\n", "GOPATH environment variable is not set or empty")
		os.Exit(2)
	}
	gopath := gps[0]
	Debugf("GOPATH: %s", gopath)

	// load config
	err := loadConfig()
	if err != nil {
		ColorLog("[ERRO] Fail to parse bee.json[ %s ]\n", err)
	}
	// getting command line arguments
	if len(args) != 0 {
		// func (f *FlagSet) Parse(arguments []string) error
		// 从arguments中解析注册的flag。
		// 必须在所有flag都注册好而未访问其值时执行。
		// 未注册却使用flag -help时，会返回ErrHelp。
		cmd.Flag.Parse(args[1:])
	}
	if mDriver == "" {
		mDriver = docValue(conf.Database.Driver)
		if mDriver == "" {
			mDriver = "mysql"
		}
	}
	if mConn == "" {
		mConn = docValue(conf.Database.Conn)
		if mConn == "" {
			mConn = "root:@tcp(127.0.0.1:3306)/test"
		}
	}
	ColorLog("[INFO] Using '%s' as 'driver'\n", mDriver)
	ColorLog("[INFO] Using '%s' as 'conn'\n", mConn)
	driverStr, connStr := string(mDriver), string(mConn)
	if len(args) == 0 {
		// run all outstanding migrations
		ColorLog("[INFO] Running all outstanding migrations\n")
		migrateUpdate(currpath, driverStr, connStr)
	} else {
		mcmd := args[0]
		switch mcmd {
		case "rollback":
			ColorLog("[INFO] Rolling back the last migration operation\n")
			migrateRollback(currpath, driverStr, connStr)
		case "reset":
			ColorLog("[INFO] Reseting all migrations\n")
			migrateReset(currpath, driverStr, connStr)
		case "refresh":
			ColorLog("[INFO] Refreshing all migrations\n")
			migrateRefresh(currpath, driverStr, connStr)
		default:
			ColorLog("[ERRO] Command is missing\n")
			os.Exit(2)
		}
	}
	ColorLog("[SUCC] Migration successful!\n")
	return 0
}

// migrateUpdate does the schema update
func migrateUpdate(currpath, driver, connStr string) {
	migrate("upgrade", currpath, driver, connStr)
}

// migrateRollback rolls back the latest migration
func migrateRollback(currpath, driver, connStr string) {
	migrate("rollback", currpath, driver, connStr)
}

// migrateReset rolls back all migrations
func migrateReset(currpath, driver, connStr string) {
	migrate("reset", currpath, driver, connStr)
}

// migrationRefresh rolls back all migrations and start over again
func migrateRefresh(currpath, driver, connStr string) {
	migrate("refresh", currpath, driver, connStr)
}

// migrate generates source code, build it, and invoke the binary who does the actual migration
func migrate(goal, currpath, driver, connStr string) {
	// func Join(elem ...string) string
	// Join函数可以将任意数量的路径元素放入一个单一路径里，会根据需要添加斜杠。
	// 结果是经过简化的，所有的空字符串元素会被忽略。
	dir := path.Join(currpath, "database", "migrations")	
	postfix := ""
	// const GOOS string = theGoos
	// GOOS是可执行程序的目标操作系统（将要在该操作系统的机器上执行）：darwin、freebsd、linux等。
	if runtime.GOOS == "windows" {
		postfix = ".exe"
	}
	binary := "m" + postfix
	source := binary + ".go"
	// connect to database
	// func Open(driverName, dataSourceName string) (*DB, error)
	// Open打开一个dirverName指定的数据库，dataSourceName指定数据源，一般包至少括数据库文件名和（可能的）连接信息。
	// 大多数用户会通过数据库特定的连接帮助函数打开数据库，返回一个*DB。
	// Go标准库中没有数据库驱动。参见http://golang.org/s/sqldrivers获取第三方驱动。
	// Open函数可能只是验证其参数，而不创建与数据库的连接。如果要检查数据源的名称是否合法，应调用返回值的Ping方法。
	// 返回的DB可以安全的被多个go程同时使用，并会维护自身的闲置连接池。这样一来，Open函数只需调用一次。很少需要关闭DB。
	db, err := sql.Open(driver, connStr)
	if err != nil {
		ColorLog("[ERRO] Could not connect to %s: %s\n", driver, connStr)
		ColorLog("[ERRO] Error: %v", err.Error())
		os.Exit(2)
	}
	// func (db *DB) Close() error
	// Close关闭数据库，释放任何打开的资源。
	// 一般不会关闭DB，因为DB句柄通常被多个go程共享，并长期活跃。
	defer db.Close()
	checkForSchemaUpdateTable(db, driver)
	latestName, latestTime := getLatestMigration(db, goal)
	writeMigrationSourceFile(dir, source, driver, connStr, latestTime, latestName, goal)
	buildMigrationBinary(dir, binary)
	runMigrationBinary(dir, binary)
	removeTempFile(dir, source)
	removeTempFile(dir, binary)
}

// checkForSchemaUpdateTable checks the existence of migrations table.
// It checks for the proper table structures and creates the table using MYSQL_MIGRATION_DDL if it does not exist.
func checkForSchemaUpdateTable(db *sql.DB, driver string) {
	showTableSQL := showMigrationsTableSQL(driver)
	// func (db *DB) Query(query string, args ...interface{}) (*Rows, error)
	// Query执行一次查询，返回多行结果（即Rows），一般用于执行select命令。
	// 参数args表示query中的占位参数。
	if rows, err := db.Query(showTableSQL); err != nil {
		ColorLog("[ERRO] Could not show migrations table: %s\n", err)
		os.Exit(2)
	} else if !rows.Next() {
		// no migrations table, create anew
		createTableSQL := createMigrationsTableSQL(driver)
		ColorLog("[INFO] Creating 'migrations' table...\n")
		if _, err := db.Query(createTableSQL); err != nil {
			ColorLog("[ERRO] Could not create migrations table: %s\n", err)
			os.Exit(2)
		}
	}

	// checking that migrations table schema are expected
	selectTableSQL := selectMigrationsTableSQL(driver)
	if rows, err := db.Query(selectTableSQL); err != nil {
		ColorLog("[ERRO] Could not show columns of migrations table: %s\n", err)
		os.Exit(2)
	} else {
		// func (rs *Rows) Next() bool
		// Next准备用于Scan方法的下一行结果。如果成功会返回真，如果没有下一行或者出现错误会返回假。
		// Err应该被调用以区分这两种情况。
		// 每一次调用Scan方法，甚至包括第一次调用该方法，都必须在前面先调用Next方法。
		for rows.Next() {
			var fieldBytes, typeBytes, nullBytes, keyBytes, defaultBytes, extraBytes []byte
			// func (rs *Rows) Scan(dest ...interface{}) error
			// Scan将当前行各列结果填充进dest指定的各个值中。
			// 如果某个参数的类型为*[]byte，Scan会保存对应数据的拷贝，该拷贝为调用者所有，可以安全的,修改或无限期的保存。
			// 如果参数类型为*RawBytes可以避免拷贝；参见RawBytes的文档获取其使用的约束。
			// 如果某个参数的类型为*interface{}，Scan会不做转换的拷贝底层驱动提供的值。
			// 如果值的类型为[]byte，会进行数据的拷贝，调用者可以安全使用该值。
			if err := rows.Scan(&fieldBytes, &typeBytes, &nullBytes, &keyBytes, &defaultBytes, &extraBytes); err != nil {
				ColorLog("[ERRO] Could not read column information: %s\n", err)
				os.Exit(2)
			}
			fieldStr, typeStr, nullStr, keyStr, defaultStr, extraStr :=
				string(fieldBytes), string(typeBytes), string(nullBytes), string(keyBytes), string(defaultBytes), string(extraBytes)
			if fieldStr == "id_migration" {
				if keyStr != "PRI" || extraStr != "auto_increment" {
					ColorLog("[ERRO] Column migration.id_migration type mismatch: KEY: %s, EXTRA: %s\n", keyStr, extraStr)
					ColorLog("[HINT] Expecting KEY: PRI, EXTRA: auto_increment\n")
					os.Exit(2)
				}
			} else if fieldStr == "name" {
				// func HasPrefix(s, prefix string) bool
				// 判断s是否有前缀字符串prefix。
				if !strings.HasPrefix(typeStr, "varchar") || nullStr != "YES" {
					ColorLog("[ERRO] Column migration.name type mismatch: TYPE: %s, NULL: %s\n", typeStr, nullStr)
					ColorLog("[HINT] Expecting TYPE: varchar, NULL: YES\n")
					os.Exit(2)
				}

			} else if fieldStr == "created_at" {
				if typeStr != "timestamp" || defaultStr != "CURRENT_TIMESTAMP" {
					ColorLog("[ERRO] Column migration.timestamp type mismatch: TYPE: %s, DEFAULT: %s\n", typeStr, defaultStr)
					ColorLog("[HINT] Expecting TYPE: timestamp, DEFAULT: CURRENT_TIMESTAMP\n")
					os.Exit(2)
				}
			}
		}
	}
}

func showMigrationsTableSQL(driver string) string {
	switch driver {
	case "mysql":
		return "SHOW TABLES LIKE 'migrations'"
	case "postgres":
		return "SELECT * FROM pg_catalog.pg_tables WHERE tablename = 'migrations';"
	default:
		return "SHOW TABLES LIKE 'migrations'"
	}
}

func createMigrationsTableSQL(driver string) string {
	switch driver {
	case "mysql":
		return MYSQLMigrationDDL
	case "postgres":
		return POSTGRESMigrationDDL
	default:
		return MYSQLMigrationDDL
	}
}

func selectMigrationsTableSQL(driver string) string {
	switch driver {
	case "mysql":
		return "DESC migrations"
	case "postgres":
		return "SELECT * FROM migrations WHERE false ORDER BY id_migration;"
	default:
		return "DESC migrations"
	}
}

// getLatestMigration retrives latest migration with status 'update'
func getLatestMigration(db *sql.DB, goal string) (file string, createdAt int64) {
	sql := "SELECT name FROM migrations where status = 'update' ORDER BY id_migration DESC LIMIT 1"
	if rows, err := db.Query(sql); err != nil {
		ColorLog("[ERRO] Could not retrieve migrations: %s\n", err)
		os.Exit(2)
	} else {
		if rows.Next() {
			if err := rows.Scan(&file); err != nil {
				ColorLog("[ERRO] Could not read migrations in database: %s\n", err)
				os.Exit(2)
			}
			createdAtStr := file[len(file)-15:]
			// func Parse(layout, value string) (Time, error)
			// Parse解析一个格式化的时间字符串并返回它代表的时间。layout定义了参考时间：
			if t, err := time.Parse("20060102_150405", createdAtStr); err != nil {
				ColorLog("[ERRO] Could not parse time: %s\n", err)
				os.Exit(2)
			} else {
				// func (t Time) Unix() int64
				// Unix将t表示为Unix时间，即从时间点January 1, 1970 UTC到时间点t所经过的时间（单位秒）。
				createdAt = t.Unix()
			}
		} else {
			// migration table has no 'update' record, no point rolling back
			if goal == "rollback" {
				ColorLog("[ERRO] There is nothing to rollback\n")
				os.Exit(2)
			}
			file, createdAt = "", 0
		}
	}
	return
}

// writeMigrationSourceFile create the source file based on MIGRATION_MAIN_TPL
func writeMigrationSourceFile(dir, source, driver, connStr string, latestTime int64, latestName string, task string) {
	changeDir(dir)
	// func OpenFile(name string, flag int, perm FileMode) (file *File, err error)
	// OpenFile是一个更一般性的文件打开函数，大多数调用者都应用Open或Create代替本函数。
	// 它会使用指定的选项（如O_RDONLY等）、指定的模式（如0666等）打开指定名称的文件。
	// 如果操作成功，返回的文件对象可用于I/O。如果出错，错误底层类型是*PathError。
	if f, err := os.OpenFile(source, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666); err != nil {
		ColorLog("[ERRO] Could not create file: %s\n", err)
		os.Exit(2)
	} else {
		// func Replace(s, old, new string, n int) string
		// 返回将s中前n个不重叠old子串都替换为new的新字符串，如果n<0会替换所有old子串。
		content := strings.Replace(MigrationMainTPL, "{{DBDriver}}", driver, -1)
		content = strings.Replace(content, "{{ConnStr}}", connStr, -1)
		content = strings.Replace(content, "{{LatestTime}}", strconv.FormatInt(latestTime, 10), -1)
		content = strings.Replace(content, "{{LatestName}}", latestName, -1)
		content = strings.Replace(content, "{{Task}}", task, -1)
		// func (f *File) WriteString(s string) (ret int, err error)
		// WriteString类似Write，但接受一个字符串参数。
		if _, err := f.WriteString(content); err != nil {
			ColorLog("[ERRO] Could not write to file: %s\n", err)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

// buildMigrationBinary changes directory to database/migrations folder and go-build the source
func buildMigrationBinary(dir, binary string) {
	changeDir(dir)
 	// func Command(name string, arg ...string) *Cmd
	// 函数返回一个*Cmd，用于使用给出的参数执行name指定的程序。返回值只设定了Path和Args两个参数。
	// 如果name不含路径分隔符，将使用LookPath获取完整路径；否则直接使用name。参数arg不应包含命令名。
	cmd := exec.Command("go", "build", "-o", binary)
	// func (c *Cmd) CombinedOutput() ([]byte, error)
	// 执行命令并返回标准输出和错误输出合并的切片。
	if out, err := cmd.CombinedOutput(); err != nil {
		ColorLog("[ERRO] Could not build migration binary: %s\n", err)
		formatShellErrOutput(string(out))
		removeTempFile(dir, binary)
		removeTempFile(dir, binary+".go")
		os.Exit(2)
	}
}

// runMigrationBinary runs the migration program who does the actual work
func runMigrationBinary(dir, binary string) {
	changeDir(dir)
	cmd := exec.Command("./" + binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		formatShellOutput(string(out))
		ColorLog("[ERRO] Could not run migration binary: %s\n", err)
		removeTempFile(dir, binary)
		removeTempFile(dir, binary+".go")
		os.Exit(2)
	} else {
		formatShellOutput(string(out))
	}
}

// changeDir changes working directory to dir.
// It exits the system when encouter an error
func changeDir(dir string) {
	// func Chdir(dir string) error
	// Chdir将当前工作目录修改为dir指定的目录。
	// 如果出错，会返回*PathError底层类型的错误。
	if err := os.Chdir(dir); err != nil {
		ColorLog("[ERRO] Could not find migration directory: %s\n", err)
		os.Exit(2)
	}
}

// removeTempFile removes a file in dir
func removeTempFile(dir, file string) {
	changeDir(dir)
	// func Remove(name string) error
	// Remove删除name指定的文件或目录。
	// 如果出错，会返回*PathError底层类型的错误。
	if err := os.Remove(file); err != nil {
		ColorLog("[WARN] Could not remove temporary file: %s\n", err)
	}
}

// formatShellErrOutput formats the error shell output
func formatShellErrOutput(o string) {
	// func Split(s, sep string) []string
	// 用去掉s中出现的sep的方式进行分割，会分割到结尾，并返回生成的所有片段组成的切片（每一个sep都会进行一次切割，即使两个sep相邻，也会进行两次切割）。
	// 如果sep为空字符，Split会将s切分成每一个unicode码值一个字符串。
	for _, line := range strings.Split(o, "\n") {
		if line != "" {
			ColorLog("[ERRO] -| ")
			fmt.Println(line)
		}
	}
}

// formatShellOutput formats the normal shell output
func formatShellOutput(o string) {
	for _, line := range strings.Split(o, "\n") {
		if line != "" {
			ColorLog("[INFO] -| ")
			fmt.Println(line)
		}
	}
}

const (
	MigrationMainTPL = `package main

import(
	"os"

	"github.com/astaxie/beego/orm"
	"github.com/astaxie/beego/migration"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func init(){
	orm.RegisterDataBase("default", "{{DBDriver}}","{{ConnStr}}")
}

func main(){
	task := "{{Task}}"
	switch task {
	case "upgrade":
		if err := migration.Upgrade({{LatestTime}}); err != nil {
			os.Exit(2)
		}
	case "rollback":
		if err := migration.Rollback("{{LatestName}}"); err != nil {
			os.Exit(2)
		}
	case "reset":
		if err := migration.Reset(); err != nil {
			os.Exit(2)
		}
	case "refresh":
		if err := migration.Refresh(); err != nil {
			os.Exit(2)
		}
	}
}

`
	MYSQLMigrationDDL = `
CREATE TABLE migrations (
	id_migration int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'surrogate key',
	name varchar(255) DEFAULT NULL COMMENT 'migration name, unique',
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'date migrated or rolled back',
	statements longtext COMMENT 'SQL statements for this migration',
	rollback_statements longtext COMMENT 'SQL statment for rolling back migration',
	status ENUM('update', 'rollback') COMMENT 'update indicates it is a normal migration while rollback means this migration is rolled back',
	PRIMARY KEY (id_migration)
) ENGINE=InnoDB DEFAULT CHARSET=utf8
`

	POSTGRESMigrationDDL = `
CREATE TYPE migrations_status AS ENUM('update', 'rollback');

CREATE TABLE migrations (
	id_migration SERIAL PRIMARY KEY,
	name varchar(255) DEFAULT NULL,
	created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
	statements text,
	rollback_statements text,
	status migrations_status
)`
)
