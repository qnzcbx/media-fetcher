package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var mixinIndex = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
	33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40,
	61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62,
	11, 36, 20, 34, 44, 52,
}

// MixinKey 根据 img_key 和 sub_key 按照置换表生成 WBI 混淆密钥。
func MixinKey(imgKey, subKey string) (string, error) {
	raw := imgKey + subKey
	if len(raw) < 64 {
		return "", errors.New("wbi: img_key + sub_key 长度不足 64 字节")
	}
	out := make([]byte, 32)
	for i := 0; i < 32; i++ {
		out[i] = raw[mixinIndex[i]]
	}
	return string(out), nil
}

// KeyFromImageURL 从哔哩哔哩 CDN 图片链接中提取哈希 Key。
func KeyFromImageURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	b := path.Base(u.Path)
	return strings.TrimSuffix(b, path.Ext(b))
}

// SignQuery 为 WBI 请求附加 wts 时间戳并计算生成 w_rid MD5 签名。
func SignQuery(q url.Values, mixinKey string, at time.Time) string {
	q.Set("wts", strconv.FormatInt(at.Unix(), 10))
	for k, values := range q {
		for i, v := range values {
			values[i] = strings.Map(func(r rune) rune {
				if strings.ContainsRune("!'()*", r) {
					return -1
				}
				return r
			}, v)
		}
		q[k] = values
	}
	plain := strings.ReplaceAll(q.Encode(), "+", "%20")
	sum := md5.Sum([]byte(plain + mixinKey))
	q.Set("w_rid", hex.EncodeToString(sum[:]))
	return strings.ReplaceAll(q.Encode(), "+", "%20")
}
