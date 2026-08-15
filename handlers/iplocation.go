package handlers

import (
	_ "embed"
	"log"
	"os"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

//go:embed ip2region.xdb
var ip2regionXDB []byte

// ipSearcher ip2region 查询器
var ipSearcher *xdb.Searcher

// InitIP2Region 初始化 ip2region 离线 IP 库
// 默认使用内嵌进二进制的库（内存模式，部署无需单独放文件，更新即重新编译）
// 若设置 IP2REGION_DB 指向外部 .xdb 文件，则优先使用外部文件（文件句柄模式，可不重编译更新库）
func InitIP2Region() {
	// 1. 外部文件优先（可选覆盖）
	if dbPath := os.Getenv("IP2REGION_DB"); dbPath != "" {
		searcher, err := xdb.NewSearcher(xdb.IPv4, dbPath, nil, nil)
		if err == nil {
			ipSearcher = searcher
			log.Printf("✅ ip2region 使用外部文件: %s (文件句柄模式)", dbPath)
			return
		}
		log.Printf("⚠️ 外部文件加载失败 (%s): %v，回退内嵌库", dbPath, err)
	}

	// 2. 内嵌库（编译进二进制，默认）
	if len(ip2regionXDB) > 0 {
		searcher, err := xdb.NewSearcher(xdb.IPv4, "", nil, ip2regionXDB)
		if err != nil {
			log.Printf("⚠️ 内嵌 ip2region 加载失败: %v", err)
			return
		}
		ipSearcher = searcher
		log.Printf("✅ ip2region 使用内嵌库 (内存模式, %d KB)", len(ip2regionXDB)/1024)
		return
	}

	log.Println("⚠️ 无可用 ip2region 库，省份校验不可用")
}

// queryProvince 查询 IP 所属省份；库不可用/查询失败返回空字符串
func queryProvince(ip string) string {
	if ipSearcher == nil {
		return ""
	}
	region, err := ipSearcher.Search(ip)
	if err != nil {
		log.Printf("⚠️ ip2region 查询失败 ip=%s: %v", ip, err)
		return ""
	}
	// 实测 v3 数据格式: 国家|省份|城市|ISP|国家代码，省份在第 2 段（index 1）
	// 例如: 中国|江苏省|南京市|0|CN
	parts := strings.Split(region, "|")
	if len(parts) >= 2 {
		province := strings.TrimSpace(parts[1])
		if province != "" && province != "0" {
			return province
		}
	}
	return ""
}
