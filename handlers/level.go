package handlers

// 统一的“用户验证等级是否达标”校验模块
//
// 规则：
//   - 用户验证等级 level == 0 视为 root，任何密钥要求都放行（例外特权）
//   - 否则须满足 level >= 开发者密钥要求的最低等级(NEL)
//
// 所有需要“等级达标”判断的地方（扫码绑定、user_verify、seeotp 等）
// 都应调用本模块，避免各处复制判断逻辑。

// userLevelSatisfies 判断用户验证等级是否满足密钥要求(NEL)
func userLevelSatisfies(userLevel, needNEL int) bool {
	// root：验证等级为 0 的用户谁都可以验证，直接放行
	if userLevel == 0 {
		return true
	}
	// NEL<=1 表示仅要求普通等级（level>=1 即满足）
	if needNEL <= 1 {
		return true
	}
	return userLevel >= needNEL
}

// devNELByToken 查询某开发者令牌要求的最低验证等级；返回 (NEL, 是否存在)
// 令牌不存在或为空（如 login OTP）时返回 false，调用方可选择跳过校验
func devNELByToken(devAccessToken string) (int, bool) {
	if devAccessToken == "" {
		return 0, false
	}
	var nel int
	err := db.QueryRow("SELECT NEL FROM devdata WHERE devaccesstoken = ?", devAccessToken).Scan(&nel)
	if err != nil {
		return 0, false
	}
	return nel, true
}
