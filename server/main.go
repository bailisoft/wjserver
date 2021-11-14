package main

import (
	"fmt"
	"lxsoft/amwj/api"
	"lxsoft/amwj/core"
	"lxsoft/amwj/data"
	"lxsoft/amwj/filter"
	"lxsoft/amwj/logx"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	//数据目录
	os.Mkdir("chains", 0775)
	os.Mkdir("images", 0775)
	os.Mkdir("thumbs", 0775)

	//以下路由区分大小写————约定全部小写

	//2B路由
	verify := filter.Verify
	http.Handle("/putshop", verify(http.HandlerFunc(api.PutShop)))
	http.Handle("/delshop", verify(http.HandlerFunc(api.DelShop)))
	http.Handle("/putimage", verify(http.HandlerFunc(api.PutImage)))
	http.Handle("/synstock", verify(http.HandlerFunc(api.SynStock)))

	//2C路由
	http.Handle("/tags", http.HandlerFunc(api.WwwAllTags))
	http.Handle("/search", http.HandlerFunc(api.WwwSearch))
	http.Handle("/shopinfo", http.HandlerFunc(api.WwwShopInfo))

	//2S路由————目前事实上未用到，暂时先直接在后台用 kill -s 1 发送信号读取文件更新。留待后期完善。
	guard := filter.Guard
	http.Handle("/senyou_wumaru_insert_label", guard(http.HandlerFunc(api.NewLabel)))
	http.Handle("/senyou_wumaru_upsert_backer", guard(http.HandlerFunc(api.UpsertBacker)))

	//启动服务
	go http.ListenAndServe(":8080", nil)
	logx.Logln("Server started successfully.")

	// //调试
	// if logx.DebugMode {
	// 	core.PrintChain("女士大衣")
	// }

	//设置系统信号
	signChan := make(chan os.Signal, 1)
	signal.Notify(signChan, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)

	//等待
	for {
		quitt := false

		select {
		case sign := <-signChan:
			if syscall.SIGHUP == sign {

				//kill -s 1 xxxxpid 更新backer与label，
				//后期完善使用 RestFul http 请求方法。 【注意】不可以使用 kill -s 9 强制终止进程！！！

				core.CheckReloadNewBackers()

				core.CheckReloadNewLabels()

				ok := updateSiteIndex()
				if !ok {
					logx.Fatalln("updateSiteIndex failed.")
					quitt = true
				}

				logx.Logln("checkReload ok")

				//shops由网络用户请求更新，不用在此更新

			} else { //os.Interrupt(syscall.SIGINT,Ctrl+C) == sign || syscall.SIGQUIT == sign || syscall.SIGTERM == sign
				//kill -s 2 xxxxpid 或 kill -s 3 xxxxpid 或 kill [-s 15] xxxxpid 或 Ctrl+C 优雅结束进程
				logx.Logf("Main Stopped by %v.", sign)
				quitt = true
			}

		case backerLine := <-data.BackerChan:
			data.SaveFileLine("./backers.txt", backerLine)

		case shopLine := <-data.ShopChan:
			data.SaveFileLine("./shops.txt", shopLine)

		case labelLine := <-data.LabelChan:
			data.SaveFileLine("./labels.txt", labelLine)

		case chainLine := <-data.ChainChan:
			flds := strings.Split(chainLine, "\f")
			if len(flds) == 2 {
				data.SaveFileLine(fmt.Sprintf("./chains/%s.txt", flds[0]), flds[1])
			}
		}

		if quitt {
			break
		}
	}
	logx.Logln("Main Stopped gracefully!\n\n\n")
}
