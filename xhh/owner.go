package xhh

import (
	"openxhh/config"
	"openxhh/loger"
	"strconv"
	"strings"
)

var Owners []int
var ownerIDsLoaded bool

func Check(UID int) bool {
	cfg := config.ConfigStruct.Xhh
	if !cfg.EnableWhitelist {
		return true
	}
	if strings.TrimSpace(cfg.Owner) == "" {
		if loger.Loger != nil {
			loger.Loger.Error("您已启用白名单，但未在配置中设置 xhh.owner")
		}
		return false
	}
	if len(ownerIDs()) == 0 {
		if loger.Loger != nil {
			loger.Loger.Error("您已启用白名单，但 xhh.owner 中没有有效 UID")
		}
		return false
	}
	return IsOwner(UID)
}

func IsOwner(UID int) bool {
	for _, v := range ownerIDs() {
		if v == UID {
			return true
		}
	}
	return false
}

func ownerIDs() []int {
	ownerIDsLoaded = true
	Owners = nil
	for _, v := range strings.Split(config.ConfigStruct.Xhh.Owner, ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			if loger.Loger != nil {
				loger.Loger.Error("[XHH]您的 owner 配置->" + v + "<-似乎并非数字")
			}
			continue
		}
		Owners = append(Owners, i)
	}
	return Owners
}
