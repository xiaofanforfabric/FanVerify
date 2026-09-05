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
	dsn := "./fanverify.db"
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
	dbName := getEnv("MYSQL_DB", "fanverify")

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
		lostdate  TEXT NOT NULL,
		UID       INTEGER DEFAULT 0,
		band_time TEXT DEFAULT '',
		devaccesstoken TEXT DEFAULT '' -- 发起 OTP 请求的开发者令牌
	)`
	if _, err := db.Exec(otpQuery); err != nil {
		log.Fatalf("创建 otpdata 表失败: %v", err)
	}

	// 创建开发者认证表
	devQuery := `CREATE TABLE IF NOT EXISTS devdata (
		id                INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		devaccesstoken    TEXT         NOT NULL UNIQUE,
		NEL               INTEGER      NOT NULL DEFAULT 1, -- 要求最低验证等级
		BUID              INTEGER      NOT NULL DEFAULT 0,
		BIP               TEXT         DEFAULT NULL,
		mode              TEXT         NOT NULL DEFAULT 'HTTP',
		Date_of_Issue     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		SME               TEXT         DEFAULT '', -- 业务描述
		status            TEXT         NOT NULL DEFAULT 'wait',
		NOV               INTEGER      NOT NULL DEFAULT 0, -- 合法的成功验证测试
		PUID              TEXT         DEFAULT '', -- 已成功验证的用户UID
		tmp_ip            TEXT         DEFAULT '',
		tmp_ip_lost_time  TEXT         DEFAULT ''
	)`
	if _, err := db.Exec(devQuery); err != nil {
		log.Fatalf("创建 devdata 表失败: %v", err)
	}

	// 创建标签表
	tagQuery := `CREATE TABLE IF NOT EXISTS tagdata (
		id         INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		tag        TEXT         NOT NULL,
		from_dev   TEXT         NOT NULL, -- devaccesstoken
		tuid       INTEGER      NOT NULL, -- 给谁打的标签
		make_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status     TEXT         NOT NULL DEFAULT 'ok', -- ok 或 Revoke
		message    TEXT         DEFAULT '' -- 为什么打的
	)`
	if _, err := db.Exec(tagQuery); err != nil {
		log.Fatalf("创建 tagdata 表失败: %v", err)
	}

	// 创建升级二级验证临时表（手机号短信验证码）
	tmpQuery := `CREATE TABLE IF NOT EXISTS tmp_biand (
		id             INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		openid         VARCHAR(64)  NOT NULL DEFAULT '',  -- 微信 openid
		phone          VARCHAR(20)  NOT NULL DEFAULT '',  -- 目标手机号
		session_token  VARCHAR(64)  NOT NULL DEFAULT '',  -- 验证会话令牌
		lost_time      VARCHAR(20)  NOT NULL DEFAULT '',  -- 验证码重发限制过期戳(60s)
		pass_code      VARCHAR(6)   NOT NULL DEFAULT '',  -- 短信验证码
		pass_lost_time VARCHAR(20)  NOT NULL DEFAULT '',  -- 验证码过期戳(5min)
		create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(tmpQuery); err != nil {
		log.Fatalf("创建 tmp_biand 表失败: %v", err)
	}
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tmp_biand_openid ON tmp_biand(openid)")

	// 创建开发者入驻申请(visa)表
	visaQuery := `CREATE TABLE IF NOT EXISTS dev_visa_list (
		id             INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		openid         VARCHAR(64)  NOT NULL DEFAULT '',
		uid            INTEGER      NOT NULL DEFAULT 0,
		type           VARCHAR(16)  NOT NULL DEFAULT 'web',   -- web/javamc/cmc/none(其他)
		other_type     VARCHAR(64)  NOT NULL DEFAULT '',      -- 选"其他"时填写的具体类型(需人工审核)
		serverip       VARCHAR(64)  NOT NULL DEFAULT '',      -- 服务器请求出口 IP(仅 IPv4)
		service_adderss VARCHAR(255) NOT NULL DEFAULT '',     -- 业务地址: IP:端口 或 域名
		connect_type   VARCHAR(8)   NOT NULL DEFAULT 'http',  -- http / wsc(正向WS) / wss(反向WS)
		tag            TEXT         NOT NULL DEFAULT '',      -- 业务备注
		status         VARCHAR(8)   NOT NULL DEFAULT 'wait',  -- wait=审核中 / ok=已通过 / ice=删除冻结
		end_time       VARCHAR(20)  NOT NULL DEFAULT '',      -- 审核结束时间戳(申请时间+1天)
		admin_see      VARCHAR(8)   NOT NULL DEFAULT 'no',    -- yes/no 是否需要人工审核
		plan_id        VARCHAR(64)  NOT NULL DEFAULT ''       -- 关联 plan_list.plan_id
	)`
	if _, err := db.Exec(visaQuery); err != nil {
		log.Fatalf("创建 dev_visa_list 表失败: %v", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_dev_visa_uid ON dev_visa_list(uid)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_dev_visa_biz ON dev_visa_list(serverip, service_adderss)")

	// 创建计划任务(保活检测)表
	planQuery := `CREATE TABLE IF NOT EXISTS plan_list (
		id            INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
		plan_id       VARCHAR(64)  NOT NULL DEFAULT '',  -- 任务标识(关联 dev_visa_list.plan_id)
		last_see_time VARCHAR(20)  NOT NULL DEFAULT '',  -- 上次检查时间戳
		output        TEXT         NOT NULL DEFAULT ''   -- "时间戳1:online/offline,时间戳2:online/offline"
	)`
	if _, err := db.Exec(planQuery); err != nil {
		log.Fatalf("创建 plan_list 表失败: %v", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_plan_id ON plan_list(plan_id)")

	// 兼容旧表：尝试添加新增字段（忽略已存在错误）
	db.Exec("ALTER TABLE userdata ADD COLUMN code_lost_time TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE userdata ADD COLUMN status VARCHAR(8) NOT NULL DEFAULT 'ok'")

	// 兼容 otpdata 表：添加 devaccesstoken 字段
	db.Exec("ALTER TABLE otpdata ADD COLUMN devaccesstoken TEXT DEFAULT ''")

	// 兼容 devdata 表：添加 PUID 字段
	db.Exec("ALTER TABLE devdata ADD COLUMN PUID TEXT DEFAULT ''")

	// UID 从 100000 开始（必须在所有 ALTER TABLE 之后，否则 ALTER 会重置 sqlite_sequence）
	db.Exec("DELETE FROM sqlite_sequence WHERE name='userdata'")
	db.Exec("INSERT INTO sqlite_sequence(name, seq) VALUES('userdata', 99999)")

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
		code_lost_time VARCHAR(20)  NOT NULL DEFAULT '',
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
		UID       INT DEFAULT 0,
		band_time VARCHAR(20) DEFAULT '',
		devaccesstoken VARCHAR(128) DEFAULT '', -- 发起 OTP 请求的开发者令牌
		INDEX idx_lostdate (lostdate),
		INDEX idx_uid (UID)
	)`
	if _, err := db.Exec(otpQuery); err != nil {
		log.Fatalf("创建 otpdata 表失败: %v", err)
	}

	// 创建开发者认证表
	devQuery := `CREATE TABLE IF NOT EXISTS devdata (
		id                INT          NOT NULL PRIMARY KEY AUTO_INCREMENT,
		devaccesstoken    VARCHAR(128) NOT NULL UNIQUE,
		NEL               INT          NOT NULL DEFAULT 1, -- 要求最低验证等级
		BUID              INT          NOT NULL DEFAULT 0,
		BIP               VARCHAR(64)  DEFAULT NULL,
		mode              VARCHAR(10)  NOT NULL DEFAULT 'HTTP',
		Date_of_Issue     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		SME               TEXT, -- 业务描述
		status            VARCHAR(10)  NOT NULL DEFAULT 'wait',
		NOV               INT          NOT NULL DEFAULT 0, -- 合法的成功验证测试
		PUID              TEXT, -- 已成功验证的用户UID
		tmp_ip            VARCHAR(64)  DEFAULT '',
		tmp_ip_lost_time  VARCHAR(20)  DEFAULT '',
		INDEX idx_devaccesstoken (devaccesstoken),
		INDEX idx_buid (BUID),
		INDEX idx_status (status)
	)`
	if _, err := db.Exec(devQuery); err != nil {
		log.Fatalf("创建 devdata 表失败: %v", err)
	}

	// 创建标签表
	tagQuery := `CREATE TABLE IF NOT EXISTS tagdata (
		id         INT          NOT NULL PRIMARY KEY AUTO_INCREMENT,
		tag        VARCHAR(128) NOT NULL,
		from_dev   VARCHAR(128) NOT NULL, -- devaccesstoken
		tuid       INT          NOT NULL, -- 给谁打的标签
		make_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status     VARCHAR(10)  NOT NULL DEFAULT 'ok', -- ok 或 Revoke
		message    TEXT, -- 为什么打的
		INDEX idx_from_dev (from_dev),
		INDEX idx_tuid (tuid),
		INDEX idx_status (status)
	)`
	if _, err := db.Exec(tagQuery); err != nil {
		log.Fatalf("创建 tagdata 表失败: %v", err)
	}

	// 创建升级二级验证临时表（手机号短信验证码）
	tmpQuery := `CREATE TABLE IF NOT EXISTS tmp_biand (
		id             INT          NOT NULL PRIMARY KEY AUTO_INCREMENT,
		openid         VARCHAR(64)  NOT NULL DEFAULT '',
		phone          VARCHAR(20)  NOT NULL DEFAULT '',
		session_token  VARCHAR(64)  NOT NULL DEFAULT '',
		lost_time      VARCHAR(20)  NOT NULL DEFAULT '',
		pass_code      VARCHAR(6)   NOT NULL DEFAULT '',
		pass_lost_time VARCHAR(20)  NOT NULL DEFAULT '',
		create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE KEY uk_tmp_biand_openid (openid)
	)`
	if _, err := db.Exec(tmpQuery); err != nil {
		log.Fatalf("创建 tmp_biand 表失败: %v", err)
	}

	// 创建开发者入驻申请(visa)表
	visaQuery := `CREATE TABLE IF NOT EXISTS dev_visa_list (
		id             INTEGER      NOT NULL PRIMARY KEY AUTO_INCREMENT,
		openid         VARCHAR(64)  NOT NULL DEFAULT '',
		uid            INT          NOT NULL DEFAULT 0,
		type           VARCHAR(16)  NOT NULL DEFAULT 'web',   -- web/javamc/cmc/none(其他)
		other_type     VARCHAR(64)  NOT NULL DEFAULT '',      -- 选"其他"时填写的具体类型(需人工审核)
		serverip       VARCHAR(64)  NOT NULL DEFAULT '',      -- 服务器请求出口 IP(仅 IPv4)
		service_adderss VARCHAR(255) NOT NULL DEFAULT '',     -- 业务地址: IP:端口 或 域名
		connect_type   VARCHAR(8)   NOT NULL DEFAULT 'http',  -- http / wsc(正向WS) / wss(反向WS)
		tag            TEXT,                                 -- 业务备注
		status         VARCHAR(8)   NOT NULL DEFAULT 'wait',  -- wait=审核中 / ok=已通过 / ice=删除冻结
		end_time       VARCHAR(20)  NOT NULL DEFAULT '',      -- 审核结束时间戳(申请时间+1天)
		admin_see      VARCHAR(8)   NOT NULL DEFAULT 'no',    -- yes/no 是否需要人工审核
		plan_id        VARCHAR(64)  NOT NULL DEFAULT '',      -- 关联 plan_list.plan_id
		INDEX idx_dev_visa_uid (uid),
		INDEX idx_dev_visa_biz (serverip, service_adderss)
	)`
	if _, err := db.Exec(visaQuery); err != nil {
		log.Fatalf("创建 dev_visa_list 表失败: %v", err)
	}

	// 创建计划任务(保活检测)表
	planQuery := `CREATE TABLE IF NOT EXISTS plan_list (
		id            INT          NOT NULL PRIMARY KEY AUTO_INCREMENT,
		plan_id       VARCHAR(64)  NOT NULL DEFAULT '',  -- 任务标识(关联 dev_visa_list.plan_id)
		last_see_time VARCHAR(20)  NOT NULL DEFAULT '',  -- 上次检查时间戳
		output        TEXT,                              -- "时间戳1:online/offline,时间戳2:online/offline"
		INDEX idx_plan_id (plan_id)
	)`
	if _, err := db.Exec(planQuery); err != nil {
		log.Fatalf("创建 plan_list 表失败: %v", err)
	}

	// 兼容旧表：尝试添加新增字段
	db.Exec("ALTER TABLE userdata ADD COLUMN code_lost_time TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE userdata ADD COLUMN status VARCHAR(8) NOT NULL DEFAULT 'ok'")

	// 兼容 otpdata 表：添加 devaccesstoken 字段
	db.Exec("ALTER TABLE otpdata ADD COLUMN devaccesstoken VARCHAR(128) DEFAULT ''")

	// 兼容 devdata 表：添加 PUID 字段
	db.Exec("ALTER TABLE devdata ADD COLUMN PUID TEXT")

	// 检查并修复 otpdata 表结构
	var columnCount int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'otpdata' AND COLUMN_NAME = 'UID'").Scan(&columnCount)
	if err == nil && columnCount == 0 {
		// UID 列不存在，添加列
		log.Println("⚠️ otpdata 表缺少 UID 列，正在修复...")
		db.Exec("ALTER TABLE otpdata ADD COLUMN UID INT DEFAULT 0")
		db.Exec("ALTER TABLE otpdata ADD COLUMN band_time VARCHAR(20) DEFAULT ''")
		db.Exec("ALTER TABLE otpdata ADD INDEX idx_lostdate (lostdate)")
		db.Exec("ALTER TABLE otpdata ADD INDEX idx_uid (UID)")
		log.Println("✅ otpdata 表结构已修复")
	}

	// 确保 UID 从 100000 开始（即使是已存在的表）
	db.Exec("ALTER TABLE userdata AUTO_INCREMENT = 100000")

	log.Println("✅ 数据表: userdata 就绪 (MySQL, UID起始 100000)")
}

// GetDB 返回全局数据库连接
func GetDB() *sql.DB {
	return db
}
