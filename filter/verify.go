package filter

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"lxsoft/amwj/core"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//2B
func Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//时间验证（误差不得超过5分钟）
		csecs := r.Header.Get("Baili-Seconds")
		iCSecs, err := strconv.ParseInt(csecs, 10, 64)
		iSSecs := time.Now().Unix()
		if err != nil || iSSecs-iCSecs > 300 || iCSecs-iSSecs > 300 {
			w.WriteHeader(499)
			return
		}

		//获取账号及密码
		backer := r.Header.Get("Baili-User")
		phash := core.GetBackerPass(backer)
		if len(phash) != 64 {
			w.WriteHeader(498)
			return
		}

		//账号密码验证
		vtext := backer + phash + csecs + r.Header.Get("Baili-Nonce")
		shash := fmt.Sprintf("%x", sha256.Sum256([]byte(vtext)))
		if r.Header.Get("Baili-Sha256") != shash {
			w.WriteHeader(497)
			return
		}

		//接收上传数据
		if r.Method == "POST" {

			//读取
			data, err := ioutil.ReadAll(io.LimitReader(r.Body, int64(core.MAX_UPLOAD_SIZE)))
			if err != nil {
				w.WriteHeader(496)
				return
			}

			//解析
			var paramsData string
			boundary := r.Header.Get("Baili-Boundary")
			if len(boundary) == 32 {
				pos := bytes.Index(data, []byte(boundary))
				paramsData = string(data[0:pos])
				r.Body = ioutil.NopCloser(bytes.NewBuffer(data[pos+32:])) //重置Body待读
			} else {
				paramsData = string(data)
			}

			//不必调用ParseForm，准备自行解析
			if r.Form == nil {
				r.Form = make(url.Values)
			}

			//解析自定义格式参数
			flds := strings.Split(paramsData, "\x02")
			for _, fld := range flds {
				pairs := strings.Split(fld, "\x01")
				if len(pairs) == 2 {
					k := pairs[0]
					v := pairs[1:]
					r.Form[k] = v
				}
			}
		} else {
			// GET method
			err := r.ParseForm()
			if err != nil {
				w.WriteHeader(496)
				return
			}
		}

		//继续
		next.ServeHTTP(w, r)
	})
}

//2S （暂未用到）
func Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r) //暂时只使用非公开path即可
	})
}
