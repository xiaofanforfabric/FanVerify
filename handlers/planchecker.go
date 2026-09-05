package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mcpinger "github.com/Raqbit/mc-pinger"
	"github.com/xrjr/mcutils/pkg/bedrock"
)

// 保活检测计划任务常量
const (
	planTotalChecks  = 12          // 每个申请共检测 12 次
	planOKThreshold  = 8           // 12 次中成功 >= 8 次视为业务有效
	planTickInterval = 60 * time.Second // 调度轮询间隔
	planCheckTimeout = 8 * time.Second  // 单次检测超时
)

// PlanVisaResult 单个申请结算结果（测满 12 次触发广播，供后续通知模块使用）
type PlanVisaResult struct {
	UID       int64  `json:"uid"`
	VisaID    int64  `json:"visa_id"`
	Type      string `json:"type"`
	ServerIP  string `json:"serverip"`
	Service   string `json:"service_adderss"`
	Tag       string `json:"tag"`
	OK        bool   `json:"ok"`   // true=通过并颁发密钥
	Token     string `json:"token"` // 通过时颁发的 devaccesstoken
	Online    int    `json:"online"` // 12 次中成功的次数
	Total     int    `json:"total"`  // 总检测次数(12)
}

// OnPlanVisaSettled 广播回调：测满 12 次结算后触发（供通知模块注册，nil 则仅日志）
var OnPlanVisaSettled func(PlanVisaResult)

var (
	planOnce       sync.Once
	planLastWindow string // 已执行过的 2 小时窗口 key(2006010215)
)

// StartPlanChecker 启动保活检测计划任务（后台常驻 goroutine，仅在 .env USER_PLAN_MODE=true 时启用）
// 防多节点冲突说明：调度本身不持应用锁，认领窗口用数据库原子 UPDATE 乐观锁：
//   每个 2 小时窗口内，last_see_time < 窗口起点 的任务才可被认领；
//   认领时 UPDATE ... WHERE last_see_time < 窗口起点，RowsAffected==1 才执行，
//   其它节点若已在本窗口写过 last_see_time 则认领失败，不会重复检测 → 避免同时写表爆炸。
func StartPlanChecker() {
	planOnce.Do(func() {
		go planCheckerLoop()
		log.Println("🕐 开发者业务保活检测计划任务已启动 (USER_PLAN_MODE=true，每 2 小时一轮)")
	})
}

func planCheckerLoop() {
	for {
		runPlanRoundIfDue(time.Now())
		time.Sleep(planTickInterval)
	}
}

// runPlanRoundIfDue 仅在偶数小时（00:00,02:00,…,22:00 所在窗口）且本进程本窗口未跑过时执行一轮
func runPlanRoundIfDue(now time.Time) {
	if now.Hour()%2 != 0 {
		return
	}
	key := now.Format("2006010215")
	if key == planLastWindow {
		return
	}
	planLastWindow = key
	processPlanRound(now)
}

// processPlanRound 处理一轮检测：遍历所有待检测计划任务并逐次检测
func processPlanRound(now time.Time) {
	winStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	ws := fmt.Sprintf("%d", winStart.UnixMilli()) // 本窗口起点时间戳(ms)
	nowMs := fmt.Sprintf("%d", now.UnixMilli())

	rows, err := db.Query(
		`SELECT p.plan_id, v.id, v.uid, v.type, v.connect_type, v.serverip, v.service_adderss, v.tag
		 FROM plan_list p
		 JOIN dev_visa_list v ON v.plan_id = p.plan_id
		 WHERE v.status = 'wait' AND (p.last_see_time = '' OR p.last_see_time < ?)`,
		ws,
	)
	if err != nil {
		log.Printf("❌ 保活检测: 查询待检任务失败: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var planID, typ, connType, serverip, saddr, tag string
		var visaID, uid int64
		if err := rows.Scan(&planID, &visaID, &uid, &typ, &connType, &serverip, &saddr, &tag); err != nil {
			log.Printf("⚠️ 保活检测: 读取任务行失败: %v", err)
			continue
		}

		// 原子认领本窗口：仅当 last_see_time 仍早于窗口起点（本窗口尚未被任何节点检测）才认领成功
		res, err := db.Exec(
			"UPDATE plan_list SET last_see_time = ? WHERE plan_id = ? AND (last_see_time = '' OR last_see_time < ?)",
			nowMs, planID, ws,
		)
		if err != nil {
			log.Printf("⚠️ 保活检测: 认领窗口失败 plan=%s: %v", planID, err)
			continue
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue // 已被其它节点/本进程认领，跳过
		}

		// 执行一次检测
		online := runOneCheck(typ, saddr)
		state := "offline"
		if online {
			state = "online"
		}

		// 读取并追加本次结果到 output（"时间戳:online/offline,..."）
		var output string
		db.QueryRow("SELECT output FROM plan_list WHERE plan_id = ?", planID).Scan(&output)
		if output != "" {
			output += ","
		}
		output += nowMs + ":" + state
		db.Exec("UPDATE plan_list SET output = ? WHERE plan_id = ?", output, planID)
		log.Printf("🔍 保活检测: visa=%d type=%s addr=%s → %s (%d)", visaID, typ, saddr, state, len(output))

		// 满 12 次 → 结算
		total, okCount := parsePlanOutput(output)
		if total >= planTotalChecks {
			settlePlan(PlanVisaResult{
				UID: uid, VisaID: visaID, Type: typ, ServerIP: serverip,
				Service: saddr, Tag: tag, OK: okCount >= planOKThreshold,
				Online: okCount, Total: total,
			}, planID, connType)
		}
	}
}

// runOneCheck 按业务类型执行一次可达性检测
func runOneCheck(typ, addr string) bool {
	switch typ {
	case "javamc":
		// Java 版 MC：第三方库获取服务器 MOTD（1.7+ 协议）
		host, port := splitAddr(addr, 25565)
		p := mcpinger.NewTimed(host, uint16(port), planCheckTimeout)
		info, err := p.Ping()
		return err == nil && info != nil
	case "cmc":
		// 基岩版 MC：第三方库执行 RakNet unconnected ping 获取 MOTD
		host, port := splitAddr(addr, 19132)
		done := make(chan error, 1)
		go func() {
			_, _, e := bedrock.Ping(host, port)
			done <- e
		}()
		select {
		case err := <-done:
			return err == nil
		case <-time.After(planCheckTimeout):
			return false
		}
	default: // web：GET 业务地址，返回 200 视为可达
		u := strings.TrimSpace(addr)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "http://" + u
		}
		client := &http.Client{Timeout: planCheckTimeout}
		resp, err := client.Get(u)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}
}

// settlePlan 测满 12 次后结算：通过则颁发 devaccesstoken（继承业务数据），否则标记 fail
func settlePlan(r PlanVisaResult, planID, connType string) {
	if r.OK {
		// 颁发开发者访问令牌，继承 visa 业务数据
		mode := "HTTP"
		switch connType {
		case "wsc":
			mode = "FWS"
		case "wss":
			mode = "RWS"
		}
		sme := fmt.Sprintf("[%s] %s", visaTypeName(r.Type), r.Tag)
		token := "dev_" + generateOTP(32)
		// devaccesstoken 唯一，极小概率冲突时重试
		for i := 0; i < 3; i++ {
			_, err := db.Exec(
				`INSERT INTO devdata (devaccesstoken, NEL, BUID, BIP, mode, SME, status)
				 VALUES (?, 1, ?, ?, ?, ?, 'ok')`,
				token, r.UID, r.ServerIP, mode, sme,
			)
			if err == nil {
				break
			}
			log.Printf("⚠️ 颁发令牌冲突/失败(重试 %d): %v", i+1, err)
			token = "dev_" + generateOTP(32)
		}
		r.Token = token
		db.Exec("UPDATE dev_visa_list SET status = 'ok' WHERE id = ?", r.VisaID)
		log.Printf("✅ 保活检测通过并颁发密钥: UID=%d visa=%d ok=%d/12 token=%s", r.UID, r.VisaID, r.Online, token)
	} else {
		db.Exec("UPDATE dev_visa_list SET status = 'fail' WHERE id = ?", r.VisaID)
		log.Printf("❌ 保活检测未达标: UID=%d visa=%d ok=%d/12 < %d，申请未通过", r.UID, r.VisaID, r.Online, planOKThreshold)
	}

	// 删除该计划任务（测满 12 次即终止）
	db.Exec("DELETE FROM plan_list WHERE plan_id = ?", planID)

	// 触发广播（供后续通知模块使用）
	if OnPlanVisaSettled != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					log.Printf("⚠️ 保活检测广播回调异常: %v", e)
				}
			}()
			OnPlanVisaSettled(r)
		}()
	}
}

// parsePlanOutput 解析 output 累计的检测结果，返回 (总次数, 成功次数)
func parsePlanOutput(out string) (total, okCount int) {
	for _, seg := range strings.Split(out, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.HasSuffix(seg, ":online") {
			okCount++
			total++
		} else if strings.HasSuffix(seg, ":offline") {
			total++
		}
	}
	return total, okCount
}

// splitAddr 拆分 "host:port"，无端口时使用默认端口
func splitAddr(addr string, defPort int) (string, int) {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndex(addr, ":"); i > 0 && i < len(addr)-1 {
		if p, err := strconv.Atoi(addr[i+1:]); err == nil && p >= 1 && p <= 65535 {
			return addr[:i], p
		}
	}
	return addr, defPort
}

// visaTypeName 业务类型中文名（供 SME 描述使用）
func visaTypeName(t string) string {
	switch t {
	case "web":
		return "网站"
	case "javamc":
		return "Java MC"
	case "cmc":
		return "基岩 MC"
	case "none":
		return "其他"
	}
	return t
}
