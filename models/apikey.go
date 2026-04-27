package models

import (
	"time"
)

type APIKey struct {
	ID          string             `json:"id"`
	AK          string             `json:"ak"`
	SK          string             `json:"sk"`
	Name        string             `json:"name"`
	Weight      int                `json:"weight"`
	Enabled     bool               `json:"enabled"`
	Functions   map[string]bool    `json:"functions"`
	Quotas      map[string]Quota   `json:"quotas"`
	FailedCount int                `json:"failed_count"`
	LastUsed    time.Time          `json:"last_used"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Quota struct {
	Limit   int  `json:"limit"`
	Used    int  `json:"used"`
	Enabled bool `json:"enabled"`
}

type KeysData struct {
	Keys []*APIKey `json:"keys"`
}

type FunctionType string

const (
	FuncT2IV40         FunctionType = "t2i_v40"
	FuncT2I46          FunctionType = "t2i_46"
	FuncT2V720         FunctionType = "t2v_720"
	FuncT2V1080        FunctionType = "t2v_1080"
	FuncI2VFirst720    FunctionType = "i2v_first_720"
	FuncI2VFirst1080   FunctionType = "i2v_first_1080"
	FuncI2VFirstTail720 FunctionType = "i2v_first_tail_720"
	FuncI2VFirstTail1080 FunctionType = "i2v_first_tail_1080"
	FuncI2VRecamera720 FunctionType = "i2v_recamera_720"
	FuncTI2VPro        FunctionType = "ti2v_pro"
)

var VideoFunctions = []FunctionType{
	FuncT2V720, FuncT2V1080, FuncI2VFirst720, FuncI2VFirst1080,
	FuncI2VFirstTail720, FuncI2VFirstTail1080, FuncI2VRecamera720, FuncTI2VPro,
}

var ImageFunctions = []FunctionType{
	FuncT2IV40, FuncT2I46,
}

var FunctionReqKeys = map[FunctionType]string{
	FuncT2IV40:          "jimeng_t2i_v40",
	FuncT2I46:           "jimeng_seedream46_cvtob",
	FuncT2V720:          "jimeng_t2v_v30",
	FuncT2V1080:         "jimeng_t2v_v30_1080",
	FuncI2VFirst720:     "jimeng_i2v_first_v30",
	FuncI2VFirst1080:    "jimeng_i2v_first_v30_1080",
	FuncI2VFirstTail720: "jimeng_i2v_first_tail_v30",
	FuncI2VFirstTail1080: "jimeng_i2v_first_tail_v30_1080",
	FuncI2VRecamera720:  "jimeng_i2v_recamera_v30",
	FuncTI2VPro:         "jimeng_ti2v_v30_pro",
}

var FunctionNames = map[FunctionType]string{
	FuncT2IV40:          "文生图4.0",
	FuncT2I46:           "生图4.6",
	FuncT2V720:          "文生视频720p",
	FuncT2V1080:         "文生视频1080p",
	FuncI2VFirst720:     "首帧视频720p",
	FuncI2VFirst1080:    "首帧视频1080p",
	FuncI2VFirstTail720: "首尾帧视频720p",
	FuncI2VFirstTail1080: "首尾帧视频1080p",
	FuncI2VRecamera720:  "运镜视频720p",
	FuncTI2VPro:         "3.0Pro视频",
}

func (k *APIKey) IsFunctionEnabled(function string) bool {
	if !k.Enabled {
		return false
	}
	if enabled, ok := k.Functions[function]; ok {
		return enabled
	}
	return false
}

func (k *APIKey) CheckVideoQuota(duration int) (bool, string) {
	quota, ok := k.Quotas["video"]
	if !ok || !quota.Enabled {
		return true, ""
	}
	if quota.Used+duration > quota.Limit {
		return false, "该密钥生视频额度已用尽"
	}
	return true, ""
}

func (k *APIKey) CheckImageQuota(count int) (bool, string) {
	quota, ok := k.Quotas["image"]
	if !ok || !quota.Enabled {
		return true, ""
	}
	if quota.Used+count > quota.Limit {
		return false, "该密钥生图片额度已用尽"
	}
	return true, ""
}

func (k *APIKey) UseVideoQuota(duration int) {
	if quota, ok := k.Quotas["video"]; ok {
		quota.Used += duration
		k.Quotas["video"] = quota
		if quota.Used >= quota.Limit {
			for _, f := range VideoFunctions {
				k.Functions[string(f)] = false
			}
		}
	}
}

func (k *APIKey) UseImageQuota(count int) {
	if quota, ok := k.Quotas["image"]; ok {
		quota.Used += count
		k.Quotas["image"] = quota
		if quota.Used >= quota.Limit {
			for _, f := range ImageFunctions {
				k.Functions[string(f)] = false
			}
		}
	}
}

func (k *APIKey) ResetQuotas() {
	if quota, ok := k.Quotas["video"]; ok {
		quota.Used = 0
		k.Quotas["video"] = quota
	}
	if quota, ok := k.Quotas["image"]; ok {
		quota.Used = 0
		k.Quotas["image"] = quota
	}
}

func (k *APIKey) ResetFunctions() {
	for f := range k.Functions {
		k.Functions[f] = true
	}
}
