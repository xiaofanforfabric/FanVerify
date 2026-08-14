package handlers

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 腾讯云 API 签名辅助函数
func sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

func hmacsha256(s, key string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(s))
	return string(hashed.Sum(nil))
}

// 需要保留的云产品 BusinessCode
var allowedBusiness = map[string]bool{
	"p_lighthouse": true,
	"p_cdb":        true,
	"p_domain":     true,
}

// 需要屏蔽的域名关键词
var blockedKeywords = []string{"fanauth.cn"}

// 账单数据存储目录
var zdDir string

func init() {
	exe, _ := os.Executable()
	zdDir = filepath.Join(filepath.Dir(exe), "zd")
	os.MkdirAll(zdDir, 0755)
}

// BillItem 单条资源账单
type BillItem struct {
	BusinessName string  `json:"business_name"`
	ResourceName string  `json:"resource_name"`
	ResourceID   string  `json:"resource_id"`
	Cost         float64 `json:"cost"`
	PayAmount    float64 `json:"pay_amount"`
	FeeBeginTime string  `json:"fee_begin_time"`
	FeeEndTime   string  `json:"fee_end_time"`
	Month        string  `json:"month"`
}

// MonthlyBill 单月账单
type MonthlyBill struct {
	TotalCost      float64    `json:"total_cost"`
	TotalPayAmount float64    `json:"total_pay_amount"`
	ItemCount      int        `json:"item_count"`
	Month          string     `json:"month"`
	Items          []BillItem `json:"items"`
}

// AggregatedBill 全部月份汇总
type AggregatedBill struct {
	TotalCost      float64       `json:"total_cost"`
	TotalPayAmount float64       `json:"total_pay_amount"`
	ItemCount      int           `json:"item_count"`
	MonthCount     int           `json:"month_count"`
	Months         []string      `json:"months"`
	Monthly        []MonthlyBill `json:"monthly"`
	Sponsor        *SponsorInfo  `json:"sponsor"`
}

// SponsorInfo 爱发电赞助统计
type SponsorInfo struct {
	TotalCount  int           `json:"total_count"`
	TotalAmount float64       `json:"total_amount"`
	List        []SponsorItem `json:"list"`
}

// SponsorItem 单条赞助记录
type SponsorItem struct {
	Name   string  `json:"name"`
	Avatar string  `json:"avatar"`
	Amount float64 `json:"amount"`
	Time   string  `json:"time"`
}

// ==================== 腾讯云 API 调用 ====================

func callTencentBillingAPI(month string) (map[string]interface{}, error) {
	secretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	secretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")
	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("环境变量 TENCENTCLOUD_SECRET_ID 或 TENCENTCLOUD_SECRET_KEY 未设置")
	}

	service := "billing"
	version := "2018-07-09"
	action := "DescribeBillDetail"
	host := "billing.tencentcloudapi.com"
	algorithm := "TC3-HMAC-SHA256"
	timestamp := time.Now().Unix()

	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	contentType := "application/json; charset=utf-8"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		contentType, host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	payload := fmt.Sprintf(`{"Offset":0,"Limit":300,"Month":"%s"}`, month)
	hashedRequestPayload := sha256hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest)

	secretDate := hmacsha256(date, "TC3"+secretKey)
	secretService := hmacsha256(service, secretDate)
	secretSigning := hmacsha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacsha256(string2sign, secretSigning)))

	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretID, credentialScope, signedHeaders, signature)

	url := "https://" + host
	httpRequest, _ := http.NewRequest("POST", url, strings.NewReader(payload))
	httpRequest.Header = map[string][]string{
		"Host":           {host},
		"X-TC-Action":    {action},
		"X-TC-Version":   {version},
		"X-TC-Timestamp": {strconv.FormatInt(timestamp, 10)},
		"Content-Type":   {contentType},
		"Authorization":  {authorization},
	}

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("请求腾讯云 API 失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %v", err)
	}
	return result, nil
}

// ==================== 过滤与聚合 ====================

func isBlocked(resourceName, resourceID string, componentConfigs []interface{}) bool {
	check := resourceName + " " + resourceID
	for _, cfg := range componentConfigs {
		if m, ok := cfg.(map[string]interface{}); ok {
			if name, ok := m["Name"].(string); ok && name == "域名" {
				if val, ok := m["Value"].(string); ok {
					check += " " + val
				}
			}
		}
	}
	for _, keyword := range blockedKeywords {
		if strings.Contains(check, keyword) {
			return true
		}
	}
	return false
}

func parseCost(s interface{}) float64 {
	switch v := s.(type) {
	case float64:
		return v
	case string:
		if v == "" || v == "-" {
			return 0
		}
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// processMonthData 拉取腾讯云 → 过滤聚合 → 返回 MonthlyBill
func processMonthData(month string) (*MonthlyBill, error) {
	rawResp, err := callTencentBillingAPI(month)
	if err != nil {
		return nil, err
	}

	respData, ok := rawResp["Response"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("腾讯云 API 返回格式异常")
	}

	if errMsg, hasErr := respData["Error"]; hasErr {
		if errMap, ok := errMsg.(map[string]interface{}); ok {
			code, _ := errMap["Code"].(string)
			message, _ := errMap["Message"].(string)
			return nil, fmt.Errorf("腾讯云 API 错误: %s - %s", code, message)
		}
	}

	detailSet, _ := respData["DetailSet"].([]interface{})

	type aggItem struct {
		BusinessName string
		ResourceName string
		ResourceID   string
		Cost         float64
		PayAmount    float64
		FeeBeginTime string
		FeeEndTime   string
	}
	aggMap := make(map[string]*aggItem)

	for _, raw := range detailSet {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		businessCode, _ := item["BusinessCode"].(string)
		if !allowedBusiness[businessCode] {
			continue
		}

		resourceName, _ := item["ResourceName"].(string)
		resourceID, _ := item["ResourceId"].(string)
		businessName, _ := item["BusinessCodeName"].(string)

		var compCfgs []interface{}
		if compSet, ok := item["ComponentSet"].([]interface{}); ok && len(compSet) > 0 {
			if firstComp, ok := compSet[0].(map[string]interface{}); ok {
				if cfgs, ok := firstComp["ComponentConfig"].([]interface{}); ok {
					compCfgs = cfgs
				}
			}
		}
		if isBlocked(resourceName, resourceID, compCfgs) {
			log.Printf("🚫 屏蔽条目: %s (%s)", resourceName, resourceID)
			continue
		}

		cost := 0.0
		pay := 0.0
		if compSet, ok := item["ComponentSet"].([]interface{}); ok {
			for _, comp := range compSet {
				if compMap, ok := comp.(map[string]interface{}); ok {
					pay += parseCost(compMap["CashPayAmount"])
					cost += parseCost(compMap["Cost"])
				}
			}
		}

		feeBegin, _ := item["FeeBeginTime"].(string)
		feeEnd, _ := item["FeeEndTime"].(string)

		if existing, ok := aggMap[resourceID]; ok {
			existing.Cost += cost
			existing.PayAmount += pay
		} else {
			aggMap[resourceID] = &aggItem{
				BusinessName: businessName,
				ResourceName: resourceName,
				ResourceID:   resourceID,
				Cost:         cost,
				PayAmount:    pay,
				FeeBeginTime: feeBegin,
				FeeEndTime:   feeEnd,
			}
		}
	}

	items := make([]BillItem, 0, len(aggMap))
	totalCost := 0.0
	totalPay := 0.0
	for _, a := range aggMap {
		a.Cost = float64(int(a.Cost*100)) / 100
		a.PayAmount = float64(int(a.PayAmount*100)) / 100
		totalCost += a.Cost
		totalPay += a.PayAmount
		items = append(items, BillItem{
			BusinessName: a.BusinessName,
			ResourceName: a.ResourceName,
			ResourceID:   a.ResourceID,
			Cost:         a.Cost,
			PayAmount:    a.PayAmount,
			FeeBeginTime: a.FeeBeginTime,
			FeeEndTime:   a.FeeEndTime,
			Month:        month,
		})
	}

	totalCost = float64(int(totalCost*100)) / 100
	totalPay = float64(int(totalPay*100)) / 100

	return &MonthlyBill{
		TotalCost:      totalCost,
		TotalPayAmount: totalPay,
		ItemCount:      len(items),
		Month:          month,
		Items:          items,
	}, nil
}

// ==================== 文件存储 ====================

func saveMonthlyBill(month string, bill *MonthlyBill) error {
	path := filepath.Join(zdDir, month+".json")
	data, err := json.MarshalIndent(bill, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadMonthlyBill(month string) (*MonthlyBill, error) {
	path := filepath.Join(zdDir, month+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bill MonthlyBill
	if err := json.Unmarshal(data, &bill); err != nil {
		return nil, err
	}
	return &bill, nil
}

func loadAllBills() ([]*MonthlyBill, error) {
	entries, err := os.ReadDir(zdDir)
	if err != nil {
		return nil, err
	}

	var bills []*MonthlyBill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		month := strings.TrimSuffix(e.Name(), ".json")
		bill, err := loadMonthlyBill(month)
		if err != nil {
			log.Printf("⚠️ 读取 %s 失败: %v", e.Name(), err)
			continue
		}
		bills = append(bills, bill)
	}
	return bills, nil
}

// ==================== 定时任务 ====================

// StartBillScheduler 启动定时任务，每月1日00:00自动拉取上月账单
func StartBillScheduler() {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.AddDate(0, 1, 0)
			}
			wait := next.Sub(now)
			log.Printf("⏰ 账单定时任务: 下次执行 %s (还有 %.0f 小时)", next.Format("2006-01-02 15:04:05"), wait.Hours())
			time.Sleep(wait)

			lastMonth := next.AddDate(0, -1, 0).Format("2006-01")
			log.Printf("⏰ 定时拉取账单: %s", lastMonth)
			bill, err := processMonthData(lastMonth)
			if err != nil {
				log.Printf("❌ 定时拉取账单失败: %v", err)
				continue
			}
			if err := saveMonthlyBill(lastMonth, bill); err != nil {
				log.Printf("❌ 定时保存账单失败: %v", err)
				continue
			}
			log.Printf("✅ 定时账单已更新: %s, 共 %d 项, 实付 %.2f 元", lastMonth, bill.ItemCount, bill.TotalPayAmount)
		}
	}()
}

// ==================== 爱发电 API ====================

// AfdianResponse 爱发电响应通用结构
type AfdianResponse struct {
	Ec   int                    `json:"ec"`
	Em   string                 `json:"em"`
	Data *AfdianSponsorData     `json:"data"`
}

type AfdianSponsorData struct {
	TotalCount int                `json:"total_count"`
	TotalPage  int                `json:"total_page"`
	List       []AfdianSponsorEl  `json:"list"`
}

type AfdianSponsorEl struct {
	AllSumAmount string      `json:"all_sum_amount"`
	LastPayTime  int64       `json:"last_pay_time"`
	User         *AfdianUser `json:"user"`
}

type AfdianUser struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// afdianSign 计算爱发电 API 签名
// sign = md5(token + "params" + params + "ts" + ts + "user_id" + user_id)
func afdianSign(token, params string, ts int64) string {
	raw := fmt.Sprintf("%sparams%sts%duser_id%s", token, params, ts, os.Getenv("AFDIAN_USER_ID"))
	hash := md5.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// afdianPost 发起带签名的爱发电 API 请求
func afdianPost(url, params string) (*AfdianResponse, error) {
	userID := os.Getenv("AFDIAN_USER_ID")
	token := os.Getenv("AFDIAN_TOKEN")
	ts := time.Now().Unix()
	sign := afdianSign(token, params, ts)

	body := map[string]interface{}{
		"user_id": userID,
		"params":  params,
		"ts":      ts,
		"sign":    sign,
	}
	bodyBytes, _ := json.Marshal(body)

	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result AfdianResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// queryAfdianSponsor 查询爱发电赞助统计
func queryAfdianSponsor() *SponsorInfo {
	userID := os.Getenv("AFDIAN_USER_ID")
	token := os.Getenv("AFDIAN_TOKEN")
	if userID == "" || token == "" {
		return &SponsorInfo{List: []SponsorItem{}}
	}

	// 请求所有页
	var allList []AfdianSponsorEl
	totalCount := 0
	totalAmount := 0.0
	page := 1

	for {
		params := fmt.Sprintf(`{"page":%d,"per_page":100}`, page)
		result, err := afdianPost("https://ifdian.net/api/open/query-sponsor", params)
		if err != nil {
			log.Printf("⚠️ 请求爱发电 API 失败: %v", err)
			break
		}
		if result.Ec != 200 || result.Data == nil {
			log.Printf("⚠️ 爱发电 API 异常: ec=%d, em=%s", result.Ec, result.Em)
			break
		}

		if page == 1 {
			totalCount = result.Data.TotalCount
		}

		for _, el := range result.Data.List {
			amt := 0.0
			if el.AllSumAmount != "" {
				amt, _ = strconv.ParseFloat(el.AllSumAmount, 64)
			}
			totalAmount += amt
			allList = append(allList, el)
		}

		if page >= result.Data.TotalPage {
			break
		}
		page++
	}

	log.Printf("❤️ 爱发电: %d 人赞助, 共计 %.2f 元", totalCount, totalAmount)

	list := make([]SponsorItem, 0, len(allList))
	for _, el := range allList {
		name := "匿名"
		avatar := ""
		if el.User != nil {
			name = el.User.Name
			avatar = el.User.Avatar
		}
		amt := 0.0
		if el.AllSumAmount != "" {
			amt, _ = strconv.ParseFloat(el.AllSumAmount, 64)
		}
		tm := ""
		if el.LastPayTime > 0 {
			tm = time.Unix(el.LastPayTime, 0).Format("2006-01-02")
		}
		list = append(list, SponsorItem{
			Name:   name,
			Avatar: avatar,
			Amount: amt,
			Time:   tm,
		})
	}

	return &SponsorInfo{
		TotalCount:  totalCount,
		TotalAmount: totalAmount,
		List:        list,
	}
}

// ==================== HTTP Handler ====================

// HandleGetMoneyList 获取全部月份汇总账单
// 如果有 month 参数则先拉取该月数据并缓存，再返回全部汇总
func HandleGetMoneyList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		month := r.URL.Query().Get("month")

		// 如果指定了月份，先拉取该月数据并缓存
		if month != "" {
			log.Printf("📊 拉取账单: %s", month)
			bill, err := processMonthData(month)
			if err != nil {
				log.Printf("❌ 拉取账单失败: %v", err)
				writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: err.Error()})
				return
			}
			if err := saveMonthlyBill(month, bill); err != nil {
				log.Printf("⚠️ 保存账单文件失败: %v", err)
			}
			log.Printf("✅ %s 账单已缓存: %d 项, 实付 %.2f 元", month, bill.ItemCount, bill.TotalPayAmount)
		}

		// 加载全部已缓存的账单
		allBills, err := loadAllBills()
		if err != nil {
			log.Printf("⚠️ 读取缓存目录失败: %v", err)
			writeJSON(w, http.StatusOK, AggregatedBill{
				Months:  []string{},
				Monthly: []MonthlyBill{},
			})
			return
		}

		if len(allBills) == 0 {
			writeJSON(w, http.StatusOK, AggregatedBill{
				Months:  []string{},
				Monthly: []MonthlyBill{},
			})
			return
		}

		// 按月份排序（升序）
		for i := 0; i < len(allBills); i++ {
			for j := i + 1; j < len(allBills); j++ {
				if allBills[i].Month > allBills[j].Month {
					allBills[i], allBills[j] = allBills[j], allBills[i]
				}
			}
		}

		// 聚合全部月份
		totalCost := 0.0
		totalPay := 0.0
		totalItems := 0
		months := make([]string, 0, len(allBills))
		for _, b := range allBills {
			totalCost += b.TotalCost
			totalPay += b.TotalPayAmount
			totalItems += b.ItemCount
			months = append(months, b.Month)
		}
		totalCost = float64(int(totalCost*100)) / 100
		totalPay = float64(int(totalPay*100)) / 100

		// 转为值类型
		monthly := make([]MonthlyBill, len(allBills))
		for i, b := range allBills {
			monthly[i] = *b
		}

		// 查询爱发电赞助数据
		sponsor := queryAfdianSponsor()

		resp := AggregatedBill{
			TotalCost:      totalCost,
			TotalPayAmount: totalPay,
			ItemCount:      totalItems,
			MonthCount:     len(allBills),
			Months:         months,
			Monthly:        monthly,
			Sponsor:        sponsor,
		}

		if sponsor != nil {
			log.Printf("✅ 汇总账单: %d 个月, 共 %d 项, 实付 %.2f 元, 赞助 %.2f 元",
				len(allBills), totalItems, totalPay, sponsor.TotalAmount)
		} else {
			log.Printf("✅ 汇总账单: %d 个月, 共 %d 项, 实付总计 %.2f 元", len(allBills), totalItems, totalPay)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
