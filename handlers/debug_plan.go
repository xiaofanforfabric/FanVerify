package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// DebugPlanRequest 计划任务调试请求（plan_id 与 visa_id 二选一）
type DebugPlanRequest struct {
	PlanID string `json:"plan_id"` // 计划任务标识（= dev_visa_list.plan_id）
	VisaID int64  `json:"visa_id"` // 申请记录 id（dev_visa_list.id）
}

// debugVisaRow 调试用的申请行信息
type debugVisaRow struct {
	ID          int64
	UID         int64
	Type        string
	ServerIP    string
	ServiceAddr string
	ConnectType string
	Tag         string
	Status      string
	PlanID      string
}

// loadDebugVisa 通过 plan_id 或 visa_id 解析申请行
func loadDebugVisa(req DebugPlanRequest) (*debugVisaRow, error) {
	if req.VisaID <= 0 && req.PlanID == "" {
		return nil, errors.New("需提供 plan_id 或 visa_id")
	}
	var row debugVisaRow
	var err error
	if req.VisaID > 0 {
		err = db.QueryRow(
			`SELECT id, uid, type, serverip, service_adderss, connect_type, tag, status, plan_id
			 FROM dev_visa_list WHERE id = ?`, req.VisaID,
		).Scan(&row.ID, &row.UID, &row.Type, &row.ServerIP, &row.ServiceAddr, &row.ConnectType, &row.Tag, &row.Status, &row.PlanID)
	} else {
		err = db.QueryRow(
			`SELECT v.id, v.uid, v.type, v.serverip, v.service_adderss, v.connect_type, v.tag, v.status, v.plan_id
			 FROM dev_visa_list v WHERE v.plan_id = ?`, req.PlanID,
		).Scan(&row.ID, &row.UID, &row.Type, &row.ServerIP, &row.ServiceAddr, &row.ConnectType, &row.Tag, &row.Status, &row.PlanID)
	}
	if err == sql.ErrNoRows {
		return nil, errors.New("未找到对应申请（请检查 plan_id / visa_id）")
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// HandleDebugCheckPlan 调试：手动触发一次单次检测（结果不写入数据库，直接返回给调用方查看）
// POST {plan_id} 或 {visa_id}
func HandleDebugCheckPlan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}
		var req DebugPlanRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}

		row, err := loadDebugVisa(req)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: err.Error()})
			return
		}

		online := runOneCheck(row.Type, row.ServiceAddr)
		result := "offline"
		if online {
			result = "online"
		}
		log.Printf("🔧 DEBUG 单次检测: visa=%d type=%s addr=%s → %s（未写库）", row.ID, row.Type, row.ServiceAddr, result)
		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"visa_id":        row.ID,
				"type":           row.Type,
				"service_addr":   row.ServiceAddr,
				"result":         result,
				"online":         online,
				"note":           "本次仅为测试，结果未写入数据库",
			},
		})
	}
}

// HandleDebugFinishPlan 调试：一键模拟该申请已满 12 次且在线达标，立即签发开发者密钥并广播/发邮件
// POST {plan_id} 或 {visa_id}
func HandleDebugFinishPlan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ApiResponse{Success: false, Error: "仅支持 POST 请求"})
			return
		}
		var req DebugPlanRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "请求解析失败"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: "JSON 格式错误"})
			return
		}

		row, err := loadDebugVisa(req)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ApiResponse{Success: false, Error: err.Error()})
			return
		}
		if row.Status != "wait" {
			writeJSON(w, http.StatusBadRequest, ApiResponse{Success: false, Error: fmt.Sprintf("该申请当前状态为 %s，仅 wait 状态可一键完成", row.Status)})
			return
		}

		// 复用正常结算逻辑：构造“满 12 次且通过”的结果 → settlePlan 会颁发密钥+更新状态+删计划+广播+发结果邮件
		result := PlanVisaResult{
			UID: row.UID, VisaID: row.ID, Type: row.Type, ServerIP: row.ServerIP,
			Service: row.ServiceAddr, Tag: row.Tag, OK: true, Online: 12, Total: 12,
		}
		settlePlan(result, row.PlanID, row.ConnectType)

		// 返回最新签发的密钥
		var token string
		_ = db.QueryRow("SELECT devaccesstoken FROM devdata WHERE BUID = ? AND status = 'ok' ORDER BY id DESC LIMIT 1", row.UID).Scan(&token)

		log.Printf("🔧 DEBUG 一键完成申请: visa=%d uid=%d token=%s（已广播）", row.ID, row.UID, token)
		writeJSON(w, http.StatusOK, ApiResponse{
			Success: true,
			Data: map[string]interface{}{
				"visa_id": row.ID,
				"token":   token,
				"message": "已模拟 12 次检测全部通过，密钥已签发并触发广播/邮件通知",
			},
		})
	}
}
