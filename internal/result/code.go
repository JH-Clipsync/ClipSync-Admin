package result

const (
	CodeSuccess int = 1200

	CodeFail            int = 1400
	CodeLoginErrorLimit int = 1401
	CodeAuthFail        int = 1402
	CodeAccessDenied    int = 1403
	CodeAPINotFound     int = 1404
	CodeParamError      int = 1405
	CodeUnauthorized    int = 1410
	CodeJWTExpired      int = 1411
	CodeJWTInvalid      int = 1412
	CodeSignInvalid    int = 1413 // 签名校验失败 / 缺少签名头
	CodeSignExpired    int = 1414 // 签名时间戳超出有效窗口
	CodeAccountDisable  int = 1430
	CodePasswordFormat  int = 1444
	CodeAccountLocked   int = 1445
	CodeAccountExists   int = 1450
	CodeAccountNotFound int = 1451
	CodeRecordExists    int = 1460
	CodeRecordNotFound  int = 1461

	CodeInternalError int = 1500
	CodeDBError       int = 1510
	CodeCacheError    int = 1511
)

var codeMessages = map[int]string{
	CodeSuccess: "成功",

	CodeFail:            "处理失败",
	CodeLoginErrorLimit: "登录错误次数已达上限",
	CodeAuthFail:        "用户名或密码错误",
	CodeAccessDenied:    "无接口权限",
	CodeAPINotFound:     "接口不存在",
	CodeParamError:      "请求参数错误",
	CodeUnauthorized:    "未登录或 Token 过期",
	CodeJWTExpired:      "TOKEN 已过期",
	CodeJWTInvalid:      "TOKEN 无效",
	CodeSignInvalid:    "签名校验失败",
	CodeSignExpired:    "签名已过期，请重新发起",
	CodeAccountDisable:  "账号已禁用",
	CodePasswordFormat:  "密码格式不符合规则",
	CodeAccountLocked:   "账号已锁定",
	CodeAccountExists:   "账号已存在",
	CodeAccountNotFound: "账号不存在",
	CodeRecordExists:    "记录已存在",
	CodeRecordNotFound:  "记录不存在",

	CodeInternalError: "内部服务错误",
	CodeDBError:       "数据库异常",
	CodeCacheError:    "缓存异常",
}

func MessageOf(code int) string {
	if m, ok := codeMessages[code]; ok {
		return m
	}
	return "未知错误"
}
