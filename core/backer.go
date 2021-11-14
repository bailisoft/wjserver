package core

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"lxsoft/amwj/data"
	"lxsoft/amwj/logx"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MAX_UPLOAD_SIZE = 1 << 20 // 2MB

// map key: kname
type Backer struct {
	kname string //键名
	phash string //密码SHA256
	cname string //中文名
	lics  int    //授权店数
}

var backerMap map[string]*Backer
var backerMut sync.RWMutex

func init() {

	backerMap = make(map[string]*Backer)

	err := loadBacker()
	if err != nil {
		logx.Logln("loadBacker failed: ", err)
	}

}

//调用处加锁
func loadBacker() error {

	file, err := os.Open("./backers.txt")
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	for {
		str, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		//手输有连续\t等空白，因此不能直接用Split()
		flds := strings.Fields(strings.TrimSpace(str))
		if len(flds) > 2 {
			operate := flds[1]
			k := flds[2]
			if len(flds) > 5 && ("I" == operate || "U" == operate) {
				var bk Backer
				bk.kname = k
				if len(flds[3]) == 64 {
					//后期功能开发用户正规注册的
					bk.phash = flds[3]
				} else {
					//初期后台直接向backers.txt添加的
					vtext := fmt.Sprintf("%s%saimeiwujia.com", k, flds[3])
					bk.phash = fmt.Sprintf("%x", sha256.Sum256([]byte(vtext)))
				}
				lics, _ := strconv.Atoi(flds[4])
				bk.lics = lics
				bk.cname = flds[5]
				backerMap[bk.kname] = &bk

				//fmt.Printf("op:%s, kname:%s, pwd:%s, count:%d, cname:%s\n", operate, k, flds[3], bk.lics, bk.cname)

			} else {
				delete(backerMap, k)
			}
		}
	}

	file.Close()

	return nil
}

func UpsertBacker(kname string, cname string, licount string, phash string) {

	var operate string
	backerMut.Lock()
	b, ok := backerMap[kname]
	if ok {
		b.phash = phash
		b.cname = cname
		operate = "U"
	} else {
		var bk Backer
		bk.kname = kname
		bk.phash = phash
		bk.cname = cname
		backerMap[bk.kname] = &bk
		operate = "I"
	}
	backerMut.Unlock()

	//通知记录
	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\n", time.Now().Format("2006-01-02 15:04:05"),
		operate, kname, phash, licount, cname)
	data.BackerChan <- line
}

func DeleteBacker(kname string) {

	//删除名下所有店
	shopMap.Range(func(k, v interface{}) bool {
		shopMap.Delete(k)
		return true
	})

	//删除
	backerMut.Lock()
	delete(backerMap, kname)
	backerMut.Unlock()

	//通知记录
	line := fmt.Sprintf("%s\tD\t%s\t-\t-\t-\n", time.Now().Format("2006-01-02 15:04:05"), kname)
	data.BackerChan <- line
}

func CheckReloadNewBackers() {
	backerMut.Lock()
	loadBacker()
	backerMut.Unlock()
}

func GetBackerPass(k string) string {

	var phash string
	backerMut.RLock()
	bk, ok := backerMap[k]
	if ok {
		phash = bk.phash
	}
	backerMut.RUnlock()
	return phash
}

func GetBackerInfo(k string) string {
	var cname string
	backerMut.RLock()
	bk, ok := backerMap[k]
	if ok {
		cname = bk.cname
	}
	backerMut.RUnlock()
	return cname
}

