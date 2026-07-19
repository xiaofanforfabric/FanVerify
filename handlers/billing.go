package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

// 需要保留的云产品 BusinessCode（服务器、域名类）
var allowedBusiness = map[string]bool{
	"p_lighthouse": true, // 轻量应用服务器
	"p_cdb":        true, // 云数据库 MySQL
	"p_domain":     true, // 域名注册
}

// 需要屏蔽的域名关键词
var blockedKeywords = []string{"fanauth.cn"}

// BillResponse 精简后的账单返回结构
type BillResponse struct {
	TotalCost      float64    `json:"total_cost"`
	TotalPayAmount float64    `json:"total_pay_amount"`
	ItemCount      int        `json:"item_count"`
	Month          string     `json:"month"`
	Items          []BillItem `json:"items"`
}

type BillItem struct {
	BusinessName string  `json:"business_name"`
	ResourceName string  `json:"resource_name"`
	ResourceID   string  `json:"resource_id"`
	Cost         float64 `json:"cost"`
	PayAmount    float64 `json:"pay_amount"`
	FeeBeginTime string  `json:"fee_begin_time"`
	FeeEndTime   string  `json:"fee_end_time"`
}

// callTencentBillingAPI 调用腾讯云 DescribeBillDetail
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

	// 步骤 1：拼接规范请求串
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

	// 步骤 2：拼接待签名字符串
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest)

	// 步骤 3：计算签名
	secretDate := hmacsha256(date, "TC3"+secretKey)
	secretService := hmacsha256(service, secretDate)
	secretSigning := hmacsha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacsha256(string2sign, secretSigning)))

	// 步骤 4：拼接 Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secretID,
		credentialScope,
		signedHeaders,
		signature)

	// 步骤 5：发起请求
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

// isBlocked 检查是否包含需要屏蔽的关键词
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

// parseCost 安全解析成本字符串
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

// HandleGetMoneyList 获取腾讯云账单（过滤后）
func HandleGetMoneyList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 GET 请求"})
			return
		}

		month := r.URL.Query().Get("month")
		if month == "" {
			// 默认查上个月
			now := time.Now()
			firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			lastMonth := firstOfMonth.AddDate(0, -1, 0)
			month = lastMonth.Format("2006-01")
		}

		log.Printf("📊 查询账单: %s", month)

		rawResp, err := callTencentBillingAPI(month)
		if err != nil {
			log.Printf("❌ 账单查询失败: %v", err)
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: err.Error()})
			return
		}

		// 解析 Response
		respData, ok := rawResp["Response"].(map[string]interface{})
		if !ok {
			writeJSON(w, http.StatusInternalServerError, ApiResponse{Success: false, Error: "腾讯云 API 返回格式异常"})
			return
		}

		// 检查是否有错误
		if errMsg, hasErr := respData["Error"]; hasErr {
			if errMap, ok := errMsg.(map[string]interface{}); ok {
				code, _ := errMap["Code"].(string)
				message, _ := errMap["Message"].(string)
				log.Printf("❌ 腾讯云 API 错误: %s - %s", code, message)
				writeJSON(w, http.StatusBadGateway, ApiResponse{Success: false, Error: fmt.Sprintf("腾讯云 API 错误: %s", message)})
				return
			}
		}

		detailSet, ok := respData["DetailSet"].([]interface{})
		if !ok {
			writeJSON(w, http.StatusOK, BillResponse{
				TotalCost: 0,
				ItemCount: 0,
				Month:     month,
				Items:     []BillItem{},
			})
			return
		}

		// 按 resource_id 聚合（同一个资源只显示一条，费用累加）
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

			// 检查是否需屏蔽
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

			// 累计 ComponentSet 中的费用
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

			// 按 resource_id 聚合
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

		// 转为列表
		items := make([]BillItem, 0, len(aggMap))
		totalCost := 0.0
		totalPay := 0.0
		for _, a := range aggMap {
			// 四舍五入保留两位小数
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
			})
		}

		totalCost = float64(int(totalCost*100)) / 100
		totalPay = float64(int(totalPay*100)) / 100

		resp := BillResponse{
			TotalCost:      totalCost,
			TotalPayAmount: totalPay,
			ItemCount:      len(items),
			Month:          month,
			Items:          items,
		}

		log.Printf("✅ 账单查询成功: %s, 共 %d 项, 总计 %.2f 元", month, len(items), totalCost)
		writeJSON(w, http.StatusOK, resp)
	}
}
