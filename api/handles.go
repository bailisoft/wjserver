package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"lxsoft/amwj/core"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

/*
	登记门店。此xy指精确经纬度乘以100000后取整值。
	i: backer, shop, x, y, addr, tele, resp
	o: none
	relates: shopNameMap
*/
func PutShop(w http.ResponseWriter, r *http.Request) {

	backer := r.FormValue("backer")
	shop := r.FormValue("shop")
	x, _ := strconv.Atoi(r.FormValue("x"))
	y, _ := strconv.Atoi(r.FormValue("y"))
	addr := r.FormValue("addr")
	tele := r.FormValue("tele")
	resp := r.FormValue("resp")

	if len(backer) < 1 || len(shop) < 1 {
		w.WriteHeader(479)
		return
	}

	if r.Header.Get("Baili-User") != backer {
		w.WriteHeader(478)
		return
	}

	ok := core.RegisterShop(backer, shop, addr, tele, resp, x, y)
	if ok {
		w.Write([]byte("OK"))
	} else {
		w.Write([]byte("EX"))
	}
}

/*
	删店,同时查找删除所有tagchain
	i: backer, shop
	o: none
	relates: shopNameMap, chainMap->chain
*/
func DelShop(w http.ResponseWriter, r *http.Request) {
	backer := r.FormValue("backer")
	shop := r.FormValue("shop")
	x, _ := strconv.Atoi(r.FormValue("x"))
	y, _ := strconv.Atoi(r.FormValue("y"))
	if len(backer) < 1 || len(shop) < 1 {
		w.WriteHeader(479)
		return
	}

	if r.Header.Get("Baili-User") != backer {
		w.WriteHeader(478)
		return
	}

	shopKey := core.HashGeokey(x, y)

	core.UnRegisterShop(shopKey, backer, shop)

	w.Write([]byte("OK"))
}

/*
	上传图片（客户后台登记货品窗口增加“上传图片”与“自动上下架”两个功能，标签为空表示停更）
	i: backer, hpcode, file
	o: none
	relates: imageDir
*/
func PutImage(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backer := r.FormValue("backer")
	hpcode := r.FormValue("hpcode")
	thumbStr := r.FormValue("thumb")
	if len(backer) < 1 || len(hpcode) < 1 {
		w.WriteHeader(479)
		return
	}

	if r.Header.Get("Baili-User") != backer {
		w.WriteHeader(478)
		return
	}

	//缩略图数据
	thumbBytes, err := base64.StdEncoding.DecodeString(thumbStr)
	if err != nil {
		w.WriteHeader(599)
		return
	}

	// 图片路径
	subdir := backer
	if len(backer) >= 3 {
		subdir = backer[:3]
	}

	//保存缩略图
	err = os.MkdirAll(fmt.Sprintf("./thumbs/%s/%s", subdir, backer), 0775)
	if err != nil {
		w.WriteHeader(598)
		return
	}
	dstThumb, err := os.Create(fmt.Sprintf("./thumbs/%s/%s/%s.jpg", subdir, backer, hpcode))
	if err != nil {
		w.WriteHeader(597)
		return
	}
	defer dstThumb.Close()
	_, err = io.Copy(dstThumb, bytes.NewReader(thumbBytes))
	if err != nil {
		w.WriteHeader(596)
		return
	}

	// 保存大图
	err = os.MkdirAll(fmt.Sprintf("./images/%s/%s", subdir, backer), 0775)
	if err != nil {
		w.WriteHeader(595)
		return
	}
	dstImage, err := os.Create(fmt.Sprintf("./images/%s/%s/%s.jpg", subdir, backer, hpcode))
	if err != nil {
		w.WriteHeader(594)
		return
	}
	defer dstImage.Close()
	_, err = io.Copy(dstImage, r.Body)
	if err != nil {
		w.WriteHeader(593)
		return
	}

	//成功返回
	w.Write([]byte("OK"))
}

/*
	同步库存（系统自动,非客户手动）
	i: backer, shop, tags, goods
	o: none
	relates: chainMap->chain
*/
func SynStock(w http.ResponseWriter, r *http.Request) {
	backer := r.FormValue("backer")
	shop := r.FormValue("shop")
	dels := r.FormValue("dels")
	tags := r.FormValue("tags")
	goods := r.FormValue("goods")
	x, _ := strconv.Atoi(r.FormValue("x"))
	y, _ := strconv.Atoi(r.FormValue("y"))

	if len(backer) < 1 || len(shop) < 1 {
		w.WriteHeader(479)
		return
	}

	if r.Header.Get("Baili-User") != backer {
		w.WriteHeader(478)
		return
	}

	w.Write([]byte("OK"))

	go core.SynStock(backer, shop, x, y, dels, tags, goods)
}

/*
	全部标签
	i: (none)
	o: tags[]
*/
func WwwAllTags(w http.ResponseWriter, r *http.Request) {

	//读取
	tags := core.AllLabels()

	//返回
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(tags)))
	w.WriteHeader(http.StatusOK)

	//字符串推荐使用io.WriteString()、字节串推荐使用w.Write()，格式化只好使用fmt.FPrintf()
	//如果是转发，可以直接传送w作为参数。比如json.NewEncoder(w)...
	//因为[]bytes(string)的转换过程多一遍内存复制操作
	io.WriteString(w, tags)
}

/*
	附近搜索
	i: tag, x, y
	o: goodstext[]
	relates: chainMap->chain
*/
func WwwSearch(w http.ResponseWriter, r *http.Request) {

	qry := r.URL.Query()

	tag := qry.Get("tag")
	x, _ := strconv.Atoi(qry.Get("x"))
	y, _ := strconv.Atoi(qry.Get("y"))
	mores, _ := strconv.Atoi(qry.Get("mores"))

	if len(tag) < 1 || mores < 0 || mores > 13 { // (60位geokey - 丢失32位精度) / 2位步进 = 14
		fmt.Printf("WwwSearch tag:%s, x:%d, y:%d, mores:%d\n", tag, x, y, mores)
		w.WriteHeader(499)
		return
	}

	//初期观察
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"), r.Header.Get("X-Forwarded-For"), tag, x, y)

	//一切为了这
	goods := core.SearchGoods(tag, x, y, mores)

	//故意慢点————不会影响其他goroutine
	msecs := 1000 + (rand.Int() % 1000)
	time.Sleep(time.Duration(msecs) * time.Millisecond)

	//返回
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(goods)))
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, goods)
}

/*
	门店详情
	i: backer, x, y
	o: shopTabSplitString
	relates: shopMap
*/
func WwwShopInfo(w http.ResponseWriter, r *http.Request) {
	qry := r.URL.Query()

	backer := qry.Get("com")
	x, _ := strconv.Atoi(qry.Get("x"))
	y, _ := strconv.Atoi(qry.Get("y"))

	if len(backer) < 1 {
		w.WriteHeader(499)
		return
	}

	//取得
	info := core.ShopInfo(x, y, backer)

	//返回
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(info)))
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, info)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

/*
	登记标签————仅供系统管理自用
	i: label
	o: none
	relates: chainMap
*/
func NewLabel(w http.ResponseWriter, r *http.Request) {

	//注：暂未用到，后期使用。暂用 kill -s 1 方法从文件更新

	tag := r.FormValue("tag")

	core.CheckGetChain(tag)

	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

/*
	登记后台————仅供系统管理自用
	i: kname, phash, licount, cname
	o: none
	relates: backerMap
*/
func UpsertBacker(w http.ResponseWriter, r *http.Request) {

	//注：暂未用到，后期使用。暂用 kill -s 1 方法从文件更新

	kname := r.FormValue("kname")
	licount := r.FormValue("licount")
	phash := r.FormValue("phash")
	cname := r.FormValue("cname")

	if len(kname) < 5 || len(phash) != 64 {
		w.WriteHeader(479)
		return
	}

	core.UpsertBacker(kname, cname, licount, phash)

	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

//31.124455, 121.368449	<-> 31.12784, 121.381187					121.368449, 31.124455 <-> 121.381187, 31.12784   中春高架到水清路
//966659118:wtw31f <-> 966659204:wtw344			00111001100111100000110000101110
//												00111001100111100000110010000100
//966659332 									00111001100111100000110100000100	lng, lat: 121.290000, 31.213000   差10多公里

//38.893388, 73.389736 <-> 48.523565, 135.646188                    73.389736, 38.893388 <-> 135.646188, 48.523565   和田到鸡西
//869091008:twujq0 <-> 1040468617:z08kn9		00110011110011010100011011000000
//												00111110000001000100101010001001

//0.0100000, -0.010000 <-> -0.0100000, 0.0100000		几内亚湾一级突变点
//447392427:ebpbpc <-> 626349396:kpbpbn			00011010101010101010101010101011
//												00100101010101010101010101010100

//45.01000, 89.99000  <-> 44.99000, 90.01000			乌鲁木齐东北300公里二级突变点
//917154475:vbpbpc  <-> 961893716:wpbpbn		00110110101010101010101010101011
//												00111001010101010101010101010100

//22.51000, 134.9900									太平洋 三级突变点
//67.51000, 134.9990									西伯利亚 三级突变点
//22.51000, 44.99990									沙特阿拉伯 三级突变点

//总结：180 多次二分值坐标附近，务必提高搜索级别（尽管会有更多数据，但没办法），每个突变点都是在其上级范围内，因此也不会有太大问题。

/*
女士大衣：
http://www.aimeiwujia.com/web/search?tag=%E5%A5%B3%E5%A3%AB%E5%A4%A7%E8%A1%A3&x=121480575&y=31240902&mores=0	上海新世界（总仓）
http://www.aimeiwujia.com/web/search?tag=%E5%A5%B3%E5%A3%AB%E5%A4%A7%E8%A1%A3&x=120171267&y=30255010&mores=0	杭州解百
http://www.aimeiwujia.com/web/search?tag=%E5%A5%B3%E5%A3%AB%E5%A4%A7%E8%A1%A3&x=114452803&y=39798314&mores=5	北京
http://www.aimeiwujia.com/web/search?tag=%E5%A5%B3%E5%A3%AB%E5%A4%A7%E8%A1%A3&x=104131919&y=30526851&mores=5	成都

自本地：121.365581,31.123391	111001100111100000110000101001111110110011111100000100000111   1037942320232644871		差异位数
一公里：121.375658,31.126550	111001100111100000110000101110101111000100011010110011010111   1037942325337238743		33
五公里：121.411765,31.143322	111001100111100000110010011011000111011100101000001000101000   1037942441710354984		38
十公里：121.466220,31.150322	111001100111100000111000010100010011010101010000000111010000   1037942846710415824		40
五十公：121.107653,31.518875	111001100111100001010011101011011110111011010010100010110010   1037944727026870450		43
百公里：120.465616,31.604166	111001100111001011011101100011000100011111000001110100101011   1037848648164842795		48
五百公：116.967542,33.669311	111001100101111101011100101001100110000111011101100001011111   1037505538824198239		50
千公里：114.366624,38.038826	111001110001011000100011010010111110000010101101111010101101   1040720967565500077		53
五千公： 65.967936,51.448423	110110001110100110101100110001110101001011010000110010010011   976888372115868819		58

因此公司应为 12 + 2 * mores 其中 mores <= 14
mores  0 次消除差异位数32位，表述：100m
mores  1 次消除差异位数34位，表述：1km
mores  2 次消除差异位数36位，表述：2km
mores  3 次消除差异位数38位，表述：5km
mores  4 次消除差异位数40位，表述：10km
mores  5 次消除差异位数42位，表述：50km
mores  6 次消除差异位数44位，表述：65km
mores  7 次消除差异位数46位，表述：80km
mores  8 次消除差异位数48位，表述：100km
mores  9 次消除差异位数50位，表述：500km
mores 10 次消除差异位数52位，表述：800km
mores 11 次消除差异位数54位，表述：1500km
mores 12 次消除差异位数56位，表述：2000km
mores 13 次消除差异位数58位，表述：5000km
mores 14 次消除差异位数60位，表述：全国

*/
