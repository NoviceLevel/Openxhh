package xhh

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"openxhh/loger"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

const (
	cookieFileMode       os.FileMode = 0600
	loginQRCodeImageSize             = 360
)

func Login() {
	Qr()
}

type data struct {
	Status  string `json:"status"`
	Msg     string `json:"msg"`
	Version string `json:"version"`
	Result  struct {
		Qrcode   string `json:"qr_url"`
		Expire   int    `json:"expire"`
		ErrMsg   string `json:"error_msg"`
		Err      string `json:"error"`
		NickName string `json:"nickname"`
	} `json:"result"`
}

func Qr() {
	fmt.Println("扫码登陆")
	Path := "/account/get_qrcode_url/"
	resp := SendReq("GET", Path, nil, "")
	if resp == nil {
		loger.Loger.Error("[XHH]无法创建请求")
		return
	}
	var resps data
	read, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		loger.Loger.Error("[XHH]Can't Read Body")
		return
	}
	err = json.Unmarshal(read, &resps)
	if err != nil {
		loger.Loger.Error("[XHH]Can't unmarshal body")
		return
	}
	qrURL := strings.TrimSpace(resps.Result.Qrcode)
	if qrURL == "" {
		loger.Loger.Error("[XHH]登录二维码为空")
		return
	}
	fmt.Println("扫码链接：", qrURL)
	qrLoginURL, err := url.Parse(qrURL)
	if err != nil || qrLoginURL.RawQuery == "" {
		loger.Loger.Error("[XHH]登录二维码地址格式异常", zap.String("qr_url", qrURL), zap.Error(err))
		return
	}
	qrStateQuery := "?" + qrLoginURL.RawQuery
	code, err := qrcode.New(qrURL, qrcode.Low)
	if err != nil {
		loger.Loger.Error("[XHH]无法生成二维码", zap.Error(err))
		return
	}
	err = code.WriteFile(loginQRCodeImageSize, "qrcode.png")
	if err != nil {
		loger.Loger.Error("[XHH]创建二维码图片失败", zap.Error(err))
		return
	}
	fmt.Println("二维码图片已保存到 qrcode.png")
	fmt.Println("手机终端如果显示变形，请优先打开 qrcode.png 扫描")
	fmt.Println("If terminal scanning fails, open Web UI /qrcode or /qrcode.png and scan the PNG image.")
	fmt.Println(renderTerminalQRCode(code, terminalColumns()))
	for {
		path := "/account/qr_state/"
		resp := SendReq("GET", path, nil, qrStateQuery)
		if resp == nil {
			loger.Loger.Error("[XHH]无法查询扫码状态")
			return
		}
		var resps data
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			loger.Loger.Error("[XHH]无法读取body")
			return
		}
		err = json.Unmarshal(data, &resps)
		if err != nil {
			_ = resp.Body.Close()
			loger.Loger.Error("[XHH]无法反序列化")
			return
		}
		fmt.Printf("\r %v | %v | %v", resps.Result.Err, resps.Result.ErrMsg, resps)
		if resps.Result.Err != "ok" {
			_ = resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		cookie := resp.Cookies()
		_ = resp.Body.Close()
		if len(cookie) < 2 {
			loger.Loger.Error("[XHH]扫码成功但未返回完整 Cookie")
			return
		}
		Info.Cookie = cookie[0].Name + "=" + cookie[0].Value + ";" + cookie[1].Name + "=" + cookie[1].Value
		Info.Cookie += GetFuckingToken()
		for _, v := range cookie {
			if v.Name == "user_heybox_id" {
				Info.HeyBoxId = v.Value
			}
		}
		Info.Time = int(time.Now().Unix())
		Jdata, err := json.Marshal(Info)
		if err != nil {
			loger.Loger.Error("[XHH]无法序列化", zap.Error(err))
			return
		}
		err = writeCookieFile("./cookie.json", Jdata)
		if err != nil {
			loger.Loger.Error("[XHH]创建Cookie失败", zap.Error(err))
			return
		}
		fmt.Printf("\n欢迎您 -> %s | Cookie已保存\n", resps.Result.NickName)
		return
	}
}

func writeCookieFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, cookieFileMode); err != nil {
		return err
	}
	return os.Chmod(path, cookieFileMode)
}

func terminalColumns() int {
	columns, err := strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS")))
	if err != nil || columns <= 0 {
		return 0
	}
	return columns
}

func renderTerminalQRCode(code *qrcode.QRCode, columns int) string {
	bits := code.Bitmap()
	if len(bits) == 0 {
		return ""
	}
	width := len(bits[0])
	if columns >= width*2 {
		return code.ToString(true)
	}
	if columns > 0 {
		return fmt.Sprintf("Terminal is too narrow for a scannable QR code (%d columns, need at least %d). Open qrcode.png or Web UI /qrcode and scan the PNG image.\n", columns, width*2)
	}
	return "Terminal width is unknown. Open qrcode.png or Web UI /qrcode and scan the PNG image.\n"
}

func renderNarrowQRCode(bits [][]bool, inverseColor bool) string {
	var buf bytes.Buffer
	for _, row := range bits {
		for _, bit := range row {
			if bit == inverseColor {
				buf.WriteString("█")
			} else {
				buf.WriteString(" ")
			}
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func renderCompactQRCode(bits [][]bool, inverseColor bool) string {
	var buf bytes.Buffer
	for y := 0; y < len(bits); y += 2 {
		top := bits[y]
		var bottom []bool
		if y+1 < len(bits) {
			bottom = bits[y+1]
		}
		for x, topBit := range top {
			bottomBit := !inverseColor
			if bottom != nil && x < len(bottom) {
				bottomBit = bottom[x]
			}
			buf.WriteString(compactQRBlock(topBit == inverseColor, bottomBit == inverseColor))
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func compactQRBlock(top, bottom bool) string {
	switch {
	case top && bottom:
		return "\u2588"
	case top:
		return "\u2580"
	case bottom:
		return "\u2584"
	default:
		return " "
	}
}

func GetFuckingToken() string {
	var rawstr []byte
	_str := strconv.Itoa(int(time.Now().Unix()))
	_md5 := md5.Sum([]byte(_str))
	rawstr = append(rawstr, _md5[:]...)
	_md5 = md5.Sum([]byte("唉？！云朵！"))
	rawstr = append(rawstr, _md5[:]...)
	_md5 = md5.Sum([]byte("哒哒哒哒哒，好想玩原神"))
	rawstr = append(rawstr, _md5[:]...)
	_md5 = md5.Sum([]byte("云！原！神！"))
	rawstr = append(rawstr, _md5[:]...)
	rawstr = append(rawstr, 0)
	str := ";x_xhh_tokenid=" + base64.StdEncoding.EncodeToString([]byte(rawstr))
	return str

}
func RSA(before string) (after string) {
	publicKey := "-----BEGIN PUBLIC KEY-----\nMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDZgjVwAiKTjZ55nG+mW6r3TSU4\nECvNYqDMIS/bhCj2QaH5GI/KZb2TBp+CBvUj9SLFnmJQ0kzHzHoGZCQ88VevCffF7JePGF9cmKQqotlfTKbV4oxV5iLz7JSG6b/Vg7AXtrTolNtWsa8HiB0tI0YClYaQlOXm4UxLeSxQwSFETwIDAQAB\n-----END PUBLIC KEY-----\n"
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil {
		loger.Loger.Error("[XHH]无法解析公钥")
		return
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		loger.Loger.Error("[XHH]无法解析为标准RSA Key")
		return
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		loger.Loger.Error("[XHH]Key似乎并非RsaKEY")
		return
	}
	c, err := rsa.EncryptPKCS1v15(nil, rsaPub, []byte(before))
	if err != nil {
		loger.Loger.Error("[XHH]加密失败", zap.Error(err))
		return
	}
	After := base64.StdEncoding.EncodeToString(c)
	return After
}
