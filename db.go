package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// 全局数据库连接
var db *sql.DB

// initDB 根据 STATUS 初始化数据库
func initDB() {
	status := getEnv("STATUS", "test")
	switch status {
	case "ok":
		initMySQL()
	case "test":
		initSQLite()
	default:
		log.Fatalf("未知的 STATUS 值: %s（应为 ok 或 test）", status)
	}
	createTables()
}

func initSQLite() {
	dsn := "./fanauth.db"
	var err error
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("打开 SQLite 失败: %v", err)
	}
	// SQLite 连接池设置
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		log.Fatalf("连接 SQLite 失败: %v", err)
	}
	log.Println("✅ 数据库: SQLite (测试模式)")
}

func initMySQL() {
	host := getEnv("MYSQL_HOST", "127.0.0.1")
	port := getEnv("MYSQL_PORT", "3306")
	user := getEnv("MYSQL_USER", "root")
	pass := getEnv("MYSQL_PASS", "")
	dbName := getEnv("MYSQL_DB", "fanauth")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		user, pass, host, port, dbName)

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("打开 MySQL 失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}
	log.Println("✅ 数据库: MySQL (生产模式)")
}

func createTables() {
	status := getEnv("STATUS", "test")
	switch status {
	case "ok":
		createMySQLTables()
	case "test":
		createSQLiteTables()
	}
}

func createSQLiteTables() {
	query := `CREATE TABLE IF NOT EXISTS userdata (
		UID           INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		reg_time      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		level         INT          NOT NULL DEFAULT 1,
		phone         VARCHAR(20)  NOT NULL DEFAULT '',
		tmp_code      VARCHAR(128) NOT NULL DEFAULT '',
		code_level    INT          NOT NULL DEFAULT 0,
		code_lost_time TEXT        NOT NULL DEFAULT '',
		status        VARCHAR(8)   NOT NULL DEFAULT 'ok',
		tag           VARCHAR(64)  NOT NULL DEFAULT '',
		openid        VARCHAR(64)  NOT NULL DEFAULT ''
	)`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("创建 userdata 表失败: %v", err)
	}

	// 创建 OTP 表
	otpQuery := `CREATE TABLE IF NOT EXISTS otpdata (
		otp       TEXT NOT NULL PRIMARY KEY,
		lostdate  TEXT NOT NULL
	)`
	if _, err := db.Exec(otpQuery); err != nil {
		log.Fatalf("创建 otpdata 表失败: %v", err)
	}

	// UID 从 100000 开始
	db.Exec("DELETE FROM sqlite_sequence WHERE name='userdata'")
	db.Exec("INSERT INTO sqlite_sequence(name, seq) VALUES('userdata', 99999)")

	// 兼容旧表：尝试添加新增字段（忽略已存在错误）
	db.Exec("ALTER TABLE userdata ADD COLUMN code_lost_time TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE userdata ADD COLUMN status VARCHAR(8) NOT NULL DEFAULT 'ok'")

	log.Println("✅ 数据表: userdata 就绪 (SQLite, UID起始 100000)")
}

func createMySQLTables() {
	query := `CREATE TABLE IF NOT EXISTS userdata (
		UID           INT          NOT NULL PRIMARY KEY AUTO_INCREMENT,
		reg_time      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		level         INT          NOT NULL DEFAULT 1,
		phone         VARCHAR(20)  NOT NULL DEFAULT '',
		tmp_code      VARCHAR(128) NOT NULL DEFAULT '',
		code_level    INT          NOT NULL DEFAULT 0,
		code_lost_time TEXT        NOT NULL DEFAULT '',
		status        VARCHAR(8)   NOT NULL DEFAULT 'ok',
		tag           VARCHAR(64)  NOT NULL DEFAULT '',
		openid        VARCHAR(64)  NOT NULL DEFAULT ''
	) AUTO_INCREMENT = 100000`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("创建 userdata 表失败: %v", err)
	}

	// 创建 OTP 表
	otpQuery := `CREATE TABLE IF NOT EXISTS otpdata (
		otp       VARCHAR(64) NOT NULL PRIMARY KEY,
		lostdate  VARCHAR(20) NOT NULL,
		INDEX idx_lostdate (lostdate)
	)`
	if _, err := db.Exec(otpQuery); err != nil {
		log.Fatalf("创建 otpdata 表失败: %v", err)
	}

	// 兼容旧表：尝试添加新增字段
	db.Exec("ALTER TABLE userdata ADD COLUMN code_lost_time TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE userdata ADD COLUMN status VARCHAR(8) NOT NULL DEFAULT 'ok'")

	log.Println("✅ 数据表: userdata 就绪 (MySQL, UID起始 100000)")
}

// GetDB 返回全局数据库连接
func GetDB() *sql.DB {
	return db
}
